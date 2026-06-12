package collectors

import (
	"encoding/json"
	"io"
)

func jsonDecoder(r io.Reader, out any) error {
	return json.NewDecoder(r).Decode(out)
}
