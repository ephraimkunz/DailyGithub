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
		{"Handle language that is not real", args{&FulfillmentReq{Result: ResultReq{Parameters: ParametersReq{Lang: "fakelanguage"}}}}, ""},
		{"Handle correct language", args{&FulfillmentReq{Result: ResultReq{Parameters: ParametersReq{Lang: "go"}}}}, "go"},
		{"Handle correct language that requires hyphens", args{&FulfillmentReq{Result: ResultReq{Parameters: ParametersReq{Lang: "apollo guidance computer"}}}}, "apollo-guidance-computer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLang(tt.args.fr); got != tt.want {
				t.Errorf("extractLang() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_minInt(t *testing.T) {
	type args struct {
		x int
		y int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"Test same number doesn't fail", args{5, 5}, 5},
		{"Test first order", args{5, 4}, 4},
		{"Test second order", args{4, 5}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minInt(tt.args.x, tt.args.y); got != tt.want {
				t.Errorf("minInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
