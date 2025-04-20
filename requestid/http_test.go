package requestid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP(t *testing.T) {
	svr := httptest.NewServer(Handler(true, http.HandlerFunc(echoRid)))
	t.Cleanup(svr.Close)

	client := &http.Client{}
	HTTPClientWithRequestID(client)
	req, err := http.NewRequest(http.MethodGet, svr.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	gotID := getReponseRid(t, resp)
	if gotID == "" {
		t.Error("wanted id, but got none")
	}

	ctx, id := ContextWithNewRequestID(context.Background())

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, svr.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	gotID = getReponseRid(t, resp)
	if gotID != id {
		t.Errorf("wanted id %s to be propogated, but got: %s", id, gotID)
	}
}

func TestUntrustedHTTP(t *testing.T) {
	svr := httptest.NewServer(Handler(false, http.HandlerFunc(echoRid)))
	t.Cleanup(svr.Close)

	client := &http.Client{}
	HTTPClientWithRequestID(client)

	ctx, id := ContextWithNewRequestID(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svr.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	gotID := getReponseRid(t, resp)
	if gotID == id {
		t.Errorf("wanted id %s to not be propogated, but it was", id)
	}
}

type ridResp struct {
	RequestID string `json:"requestID,omitempty"`
}

func echoRid(w http.ResponseWriter, r *http.Request) {
	id, _ := FromContext(r.Context())
	if err := json.NewEncoder(w).Encode(&ridResp{
		RequestID: id,
	}); err != nil {
		panic(err)
	}
}

func getReponseRid(t *testing.T, resp *http.Response) string {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("non OK response: %s", resp.Status)
	}
	var r ridResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("failed decoding response body: %v", err)
	}
	return r.RequestID
}
