package detect

import "encoding/json"

func jsonUnmarshal(data json.RawMessage, out any) error { return json.Unmarshal(data, out) }
