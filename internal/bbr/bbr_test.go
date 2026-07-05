package bbr

import "testing"

func TestInterfaceFromRouteOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "route get",
			output: "1.1.1.1 via 192.0.2.1 dev eth0 src 192.0.2.10 uid 0",
			want:   "eth0",
		},
		{
			name:   "default route",
			output: "default via 192.0.2.1 dev ens3 proto dhcp src 192.0.2.10",
			want:   "ens3",
		},
		{
			name:   "missing device",
			output: "default via 192.0.2.1",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := interfaceFromRouteOutput(tt.output); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
