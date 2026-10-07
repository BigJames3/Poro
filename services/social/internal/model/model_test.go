package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageSize(t *testing.T) {
	cases := map[int]int{-3: DefaultPageSize, 0: DefaultPageSize, 1: 1, 20: 20, 50: 50, 51: MaxPageSize, 1000: MaxPageSize}
	for in, want := range cases {
		require.Equal(t, want, PageSize(in), "PageSize(%d)", in)
	}
}
