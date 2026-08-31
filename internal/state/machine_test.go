package state

import "testing"

func TestParseData(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Data
	}{
		{"empty", "", Data{}},
		{"empty object", "{}", Data{}},
		{"valid", `{"age":"30"}`, Data{"age": "30"}},
		{"invalid", "not json", Data{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseData(tt.raw)
			if len(got) != len(tt.want) {
				t.Errorf("ParseData() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseData()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestDataSetGet(t *testing.T) {
	d := Data{}.Set("key", "value")
	if d.Get("key") != "value" {
		t.Errorf("Get() = %v, want value", d.Get("key"))
	}
	if d.JSON() == "{}" {
		t.Error("JSON() should not be empty")
	}
}
