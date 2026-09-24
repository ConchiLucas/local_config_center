package testhub

import (
	"strings"
	"testing"
)

func TestImagePromptUnicodeBoundary(t *testing.T) {
	for _, size := range []int{20000, 20001} {
		err := validateInput(Binding{Kind: "image"}, CreateTestRequest{Input: TestInput{Prompt: strings.Repeat("😀", size)}})
		if (err == nil) != (size == 20000) {
			t.Errorf("prompt %d runes: %v", size, err)
		}
	}
}
