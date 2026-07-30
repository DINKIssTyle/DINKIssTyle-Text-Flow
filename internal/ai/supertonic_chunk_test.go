//go:build darwin || windows

package ai

import (
	"strings"
	"testing"
)

func TestChunkTextKeepsKoreanSummaryInSourceOrder(t *testing.T) {
	text := "윗선이 원하는 바를 모두 알고 있으며, 기자들의 연임 개헌 관련 질문에 대해 " +
		"'현행 헌법상 불가능하다'는 답변을 했음에도 불구하고, 6선 국회의원 중 한 명이 " +
		"국민 뜻에 따라 개헌을 추진하는 상황은 우연이 아닌 것으로 보인다."

	chunks := chunkText(text, 120)
	firstPhrase := "윗선이 원하는 바를 모두 알고 있으며"
	if len(chunks) == 0 || !strings.HasPrefix(chunks[0], firstPhrase) {
		t.Fatalf("first phrase moved behind a later chunk: %#v", chunks)
	}
}

func TestChunkTextFlushesPendingTextBeforeLongCommaPart(t *testing.T) {
	chunks := chunkText(
		"first, second segment is definitely too long, third.",
		10,
	)
	want := []string{
		"first",
		"second",
		"segment is",
		"definitely",
		"too long",
		"third.",
	}

	if strings.Join(chunks, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("unexpected chunk order:\nwant: %#v\n got: %#v", want, chunks)
	}
}
