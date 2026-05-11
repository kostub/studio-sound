package health

import (
	"encoding/json"
	"io"
)

const Version = "0.0.1"

type Response struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func NewResponse() Response {
	return Response{
		Status:  "ok",
		Version: Version,
	}
}

func Write(w io.Writer) error {
	payload, err := json.Marshal(NewResponse())
	if err != nil {
		return err
	}

	_, err = w.Write(payload)
	return err
}
