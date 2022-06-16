package l2discovery

import "testing"

func TestSameNic(t *testing.T) {
	type args struct {
		ifaceName1 *PtpIf
		ifaceName2 *PtpIf
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{name: "ok",
			args: args{ifaceName1: &PtpIf{IfClusterIndex: IfClusterIndex{IfName: "ens3f0", NodeName: "node1"}, MacAddress: "11:11:11:11:11:11"},
				ifaceName2: &PtpIf{IfClusterIndex: IfClusterIndex{IfName: "ens3f1", NodeName: "node1"}, MacAddress: "11:11:11:11:11:11"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameNic(tt.args.ifaceName1, tt.args.ifaceName2); got != tt.want {
				t.Errorf("SameNic() = %v, want %v", got, tt.want)
			}
		})
	}
}
