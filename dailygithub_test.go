package main

import (
	_ "net/http/pprof"
	"testing"
)

func Test_extractLang(t *testing.T) {
	type args struct {
		fr *FulfillmentReq
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Handles default value lang parameter", args{&FulfillmentReq{}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLang(tt.args.fr); got != tt.want {
				t.Errorf("extractLang() = %v, want %v", got, tt.want)
			}
		})
	}
}
