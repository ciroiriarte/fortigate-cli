package protocol

import "testing"

func TestDecodeData_ExtractsResultsArray(t *testing.T) {
	body := []byte(`{"results":[{"name":"web"},{"name":"db"}],"status":"success","http_status":200}`)
	var out []struct {
		Name string `json:"name"`
	}
	if err := DecodeData(body, &out); err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	if len(out) != 2 || out[0].Name != "web" || out[1].Name != "db" {
		t.Fatalf("unexpected results: %+v", out)
	}
}

func TestDecodeData_NoEnvelopeFallsBackToWholeBody(t *testing.T) {
	body := []byte(`{"name":"web"}`)
	var out struct {
		Name string `json:"name"`
	}
	if err := DecodeData(body, &out); err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	if out.Name != "web" {
		t.Fatalf("want web, got %q", out.Name)
	}
}

func TestDecodeError_Kinds(t *testing.T) {
	cases := []struct {
		status int
		want   Kind
	}{
		{401, KindAuth},
		{403, KindAuth},
		{404, KindNotFound},
		{424, KindNotFound},
		{409, KindConflict},
		{500, KindServer},
	}
	for _, c := range cases {
		got := DecodeError(c.status, []byte(`{"status":"error","http_status":0}`))
		if got.Kind != c.want {
			t.Errorf("status %d: want kind %q, got %q", c.status, c.want, got.Kind)
		}
		if got.HTTPStatus != c.status {
			t.Errorf("status %d: HTTPStatus not preserved (%d)", c.status, got.HTTPStatus)
		}
	}
}

func TestDecodeError_UsesCLIError(t *testing.T) {
	got := DecodeError(500, []byte(`{"status":"error","cli_error":"boom","error":-3}`))
	if got.Message != "boom" {
		t.Errorf("want cli_error message, got %q", got.Message)
	}
	if got.Code != -3 {
		t.Errorf("want error code -3, got %d", got.Code)
	}
}
