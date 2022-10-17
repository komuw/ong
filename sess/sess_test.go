package sess

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

func TestSess(t *testing.T) {
	t.Parallel()

	t.Run("initialise", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		req = Initialise(req, "secretKey")

		res := req.Context().Value(ctxKey).(map[string]string)
		attest.Equal(t, res, map[string]string{})
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		k := "name"
		v := "John Keypoole"

		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		req = Initialise(req, "secretKey")

		Set(req, k, v)
		res := req.Context().Value(ctxKey).(map[string]string)
		attest.Equal(t, res, map[string]string{k: v})
	})

	t.Run("setM", func(t *testing.T) {
		t.Parallel()

		m := M{"name": "John Doe", "age": "99"}

		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		req = Initialise(req, "secretKey")

		SetM(req, m)
		res := req.Context().Value(ctxKey).(map[string]string)
		attest.Equal(t, res, m)
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		k := "name"
		v := "John Keypoole"
		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		req = Initialise(req, "secretKey")

		{
			one := Get(req, k)
			attest.Zero(t, one)

			Set(req, k, v)
			res := req.Context().Value(ctxKey).(map[string]string)
			attest.Equal(t, res, map[string]string{k: v})
		}

		{
			res := Get(req, k)
			attest.Equal(t, res, v)
		}
	})

	t.Run("getM", func(t *testing.T) {
		t.Parallel()

		m := M{"name": "John Doe", "age": "99"}
		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		req = Initialise(req, "secretKey")

		{
			one := GetM(req)
			attest.Equal(t, len(one), 0)

			SetM(req, m)
			res := req.Context().Value(ctxKey).(map[string]string)
			attest.Equal(t, res, m)
		}
		{
			res := GetM(req)
			attest.Equal(t, res, m)
		}
		{
			// returned map does not mutate the original.
			res := GetM(req)
			attest.Equal(t, res, m)
			newKey := "JJ-Masiga"
			res[newKey] = "footballer"

			newRes := Get(req, newKey)
			attest.Zero(t, newRes)
		}
	})

	t.Run("save", func(t *testing.T) {
		t.Parallel()

		m := M{"name": "John Doe", "age": "99"}
		req, err := http.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Ok(t, err)
		rec := httptest.NewRecorder()
		req = Initialise(req, "secretKey")

		{
			SetM(req, m)
			res := req.Context().Value(ctxKey).(map[string]string)
			attest.Equal(t, res, m)
		}
		{
			Save(req, rec, "localhost", 2*time.Hour, "secretKey")
		}
	})
}
