package blameworthy

import (
	"fmt"
	"testing"
)

func TestIndexing(t *testing.T) {
	var tests = []struct {
		inputCommits FileHistory
		expectedOutput string
	}{{
		FileHistory{},
		"[]",
	}, {
		FileHistory{
			FileCommit{"a1", []Hunk{
				Hunk{0,0,1,3},
			}},
		},
		"[[{3 a1}]]",
	}, {
		FileHistory{
			FileCommit{"a1", []Hunk{
				Hunk{0,0,1,3},
			}},
			FileCommit{"b2", []Hunk{
				Hunk{1,0,2,2},
				Hunk{2,0,5,2},
			}},
			FileCommit{"c3", []Hunk{
				Hunk{1,1,1,0},
				Hunk{4,2,3,1},
			}},
		},
		"[[{3 a1}]" +
			" [{1 a1} {2 b2} {1 a1} {2 b2} {1 a1}]" +
			" [{2 b2} {1 c3} {1 b2} {1 a1}]]",
	}, {
		FileHistory{
			FileCommit{"a1", []Hunk{
				Hunk{0,0,1,3},
			}},
			FileCommit{"b2", []Hunk{
				Hunk{1,1,0,0},  // remove 1st line
				Hunk{2,0,2,1},  // add new line 2
			}},
		},
		"[[{3 a1}] [{1 a1} {1 b2} {1 a1}]]",
	}, {
		FileHistory{
			FileCommit{"a1", []Hunk{
				Hunk{0,0,1,3},
			}},
			FileCommit{"b2", []Hunk{
				Hunk{1,3,0,0},
			}},
		},
		"[[{3 a1}] []]",
	}, {
		FileHistory{
			FileCommit{"a1", []Hunk{
				Hunk{0,0,1,3},
			}},
			FileCommit{"b2", []Hunk{
				Hunk{0,0,4,1},
			}},
		},
		"[[{3 a1}] [{3 a1} {1 b2}]]",
	}}
	for test_number, test := range tests {
		segments := BlameSegments{}
		out := []BlameSegments{}
		for _, commit := range test.inputCommits {
			segments = commit.step(segments)
			out = append(out, segments)
		}
		if (fmt.Sprint(out) != test.expectedOutput) {
			t.Error("Test", test_number + 1, "failed",
				"\n  Wanted", test.expectedOutput,
				"\n  Got   ", fmt.Sprint(out),
				"\n  From  ", test.inputCommits)
		}
	}
}
