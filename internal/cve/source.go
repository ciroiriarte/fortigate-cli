package cve

import (
	"context"
	"errors"
	"fmt"
)

// New builds a Source for the given mode:
//   - "nvd":   NVD only (errors surface).
//   - "circl": CIRCL only.
//   - "auto" (default, also ""): NVD first; on a rate-limit or unavailable
//     (network/5xx) error, fall back to CIRCL.
//
// nvdKey is the optional NVD API key (may be "").
func New(mode, nvdKey string) (Source, error) {
	switch mode {
	case "", "auto":
		return &autoSource{nvd: NewNVD(nvdKey), circl: NewCIRCL()}, nil
	case "nvd":
		return NewNVD(nvdKey), nil
	case "circl":
		return NewCIRCL(), nil
	default:
		return nil, fmt.Errorf("invalid --source %q (want auto|nvd|circl)", mode)
	}
}

// autoSource queries NVD first and falls back to CIRCL when NVD is rate-limited
// or unavailable. It records which backend answered so the CLI can report it.
type autoSource struct {
	nvd, circl Source
	used       string // backend that answered the most recent call
	fellBack   bool   // whether the answer came via CIRCL fallback
}

// Name reports the backend that answered the most recent call.
func (a *autoSource) Name() string {
	if a.used != "" {
		return a.used
	}
	return "auto"
}

// Note returns a human string naming the backend used, flagging a fallback.
func (a *autoSource) Note() string {
	if a.fellBack {
		return a.Name() + " (fallback)"
	}
	return a.Name()
}

// shouldFallBack reports whether an NVD error warrants trying CIRCL.
func shouldFallBack(err error) bool {
	return errors.Is(err, ErrRateLimited) || errors.Is(err, ErrUnavailable)
}

func (a *autoSource) ForVersion(ctx context.Context, fortiosVersion string) ([]CVE, error) {
	a.used, a.fellBack = "", false
	out, err := a.nvd.ForVersion(ctx, fortiosVersion)
	if err == nil {
		a.used = a.nvd.Name()
		return out, nil
	}
	if !shouldFallBack(err) {
		return nil, err
	}
	out, cerr := a.circl.ForVersion(ctx, fortiosVersion)
	if cerr != nil {
		return nil, fmt.Errorf("nvd unavailable (%v); circl fallback failed: %w", err, cerr)
	}
	a.used, a.fellBack = a.circl.Name(), true
	return out, nil
}

func (a *autoSource) ByID(ctx context.Context, id, fortiosVersion string) (CVE, error) {
	a.used, a.fellBack = "", false
	c, err := a.nvd.ByID(ctx, id, fortiosVersion)
	if err == nil {
		a.used = a.nvd.Name()
		return c, nil
	}
	if !shouldFallBack(err) {
		return CVE{}, err
	}
	c, cerr := a.circl.ByID(ctx, id, fortiosVersion)
	if cerr != nil {
		return CVE{}, fmt.Errorf("nvd unavailable (%v); circl fallback failed: %w", err, cerr)
	}
	a.used, a.fellBack = a.circl.Name(), true
	return c, nil
}

// Noter is implemented by sources that can describe which backend answered
// (including a fallback marker). Single-backend sources satisfy it via their
// Name(); autoSource adds the fallback annotation.
type Noter interface {
	Note() string
}

// SourceNote returns a display string for the backend that answered: an
// autoSource reports fallbacks, a plain source reports its Name().
func SourceNote(s Source) string {
	if n, ok := s.(Noter); ok {
		return n.Note()
	}
	return s.Name()
}
