#
# spec file for package fortigate-cli
#
# Builds the static `fgt` binary from vendored Go modules, offline, in the
# network-restricted OBS chroot. Version/commit/date are stamped via ldflags
# because .git is not present in the build root.
#

%global import_path github.com/ciroiriarte/fortigate-cli

Name:           fortigate-cli
Version:        0
Release:        0
Summary:        Unofficial remote CLI for FortiGate/FortiOS
License:        Apache-2.0
URL:            https://github.com/ciroiriarte/fortigate-cli
Source0:        %{name}-%{version}.tar
Source1:        vendor.tar.gz
BuildRequires:  golang(API) >= 1.22
BuildRequires:  go >= 1.26
BuildRequires:  fdupes
%if 0%{?suse_version} || 0%{?sle_version}
BuildRequires:  bash-completion
%endif
Provides:       fgt = %{version}-%{release}

%description
fgt is an unofficial, remote-first command-line client for the FortiGate /
FortiOS REST API (/api/v2/). It talks to the appliance over HTTPS only; nothing
is installed on the device. cmdb paths configure, monitor paths report status.

Not affiliated with or endorsed by Fortinet, Inc.

%prep
%autosetup -n %{name}-%{version}
# Unpack the pre-vendored dependencies committed alongside the package.
tar -xf %{SOURCE1}

%build
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor -buildvcs=false"
# go.mod pins `toolchain go1.26.6`; force the chroot's local toolchain so the
# offline build never tries to download a toolchain over the (blocked) network.
export GOTOOLCHAIN=local
go build -trimpath \
  -ldflags "-s -w \
    -X %{import_path}/internal/version.Version=%{version} \
    -X %{import_path}/internal/version.Commit=obs \
    -X %{import_path}/internal/version.Date=%{?SOURCE_DATE_EPOCH}" \
  -o fgt ./cmd/fgt

%check
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor -buildvcs=false"
export GOTOOLCHAIN=local
go test ./...

%install
install -Dpm0755 fgt %{buildroot}%{_bindir}/fgt
install -dpm0755 %{buildroot}%{_mandir}/man1
install -pm0644 docs/man/*.1 %{buildroot}%{_mandir}/man1/
install -Dpm0644 contrib/completions/fgt.bash %{buildroot}%{_datadir}/bash-completion/completions/fgt
install -Dpm0644 contrib/completions/fgt.zsh  %{buildroot}%{_datadir}/zsh/site-functions/_fgt
install -Dpm0644 contrib/completions/_fgt.fish %{buildroot}%{_datadir}/fish/vendor_completions.d/fgt.fish
%fdupes %{buildroot}%{_mandir}

%files
%license LICENSE NOTICE
%doc README.md
%{_bindir}/fgt
%{_mandir}/man1/fgt*.1*
%{_datadir}/bash-completion/completions/fgt
%dir %{_datadir}/zsh
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_fgt
%dir %{_datadir}/fish
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/fgt.fish

%changelog
