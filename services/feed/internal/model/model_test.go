package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageSize(t *testing.T) {
	for in, want := range map[int]int{-1: 10, 0: 10, 1: 1, 30: 30, 31: 30, 500: 30} {
		require.Equal(t, want, PageSize(in), "PageSize(%d)", in)
	}
}
