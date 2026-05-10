package probe

import (
	"encoding/json"
	"testing"
)

func FuzzParseResult(f *testing.F) {
	// Seed corpus: valid and malformed ffprobe-like JSON blobs
	seeds := []string{
		`{"format":{},"streams":[]}`,
		`{"format":{"format_name":"matroska,webm","duration":"123.45","bit_rate":"5000000","size":"1048576","probe_score":100},"streams":[]}`,
		`{"format":{"format_name":"mp4","duration":"bad","bit_rate":"bad","size":"bad","probe_score":-1},"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"bit_rate":"2000000","duration":"60.0"},{"codec_type":"audio","codec_name":"aac","channels":2,"sample_rate":"48000","bit_rate":"128000","tags":{"language":"eng"}},{"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"fra"}}]}`,
		`{}`,
		`{"format":null,"streams":null}`,
		`{"format":{"duration":"","bit_rate":"","size":""},"streams":[{"codec_type":"video","bit_rate":"","duration":"","width":0,"height":0}]}`,
		`[]`,
		`{"streams":[{"codec_type":"unknown","codec_name":"???"}]}`,
		`{"format":{"probe_score":999999999,"duration":"1e309","bit_rate":"1e309","size":"1e309"},"streams":[{"codec_type":"video","bit_rate":"1e309","duration":"1e309","width":-1,"height":-1}]}`,
		`malformed json {`,
		`{"format":{"duration":"NaN","bit_rate":"Infinity","size":"-0"},"streams":[]}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// We fuzz the JSON that would be unmarshaled into FFProbeResult.
		var result FFProbeResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			// Malformed JSON is expected; skip further processing.
			return
		}
		// parseResult must not panic on any unmarshaled input.
		_ = parseResult(&result)
	})
}
