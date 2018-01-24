package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/livegrep/livegrep/blameworthy"
	"github.com/livegrep/livegrep/server/config"
)

// Blame experiment.

type BlameData struct {
	PreviousCommit string
	NextCommit string
	Author string
	Date string
	Subject string
	Lines []BlameLine
}

type BlameLine struct {
	PreviousCommit string
	PreviousLineNumber int
	NextCommit string
	NextLineNumber int
	OldLineNumber int
	NewLineNumber int
	Symbol string
}

const blankHash = "                " // as wide as a displayed hash
var histories = make(map[string]*blameworthy.GitHistory)

func InitBlame(cfg *config.Config) (error) {
	for _, r := range cfg.IndexConfig.Repositories {
		blame, ok := r.Metadata["blame"]
		if !ok {
			continue;
		}
		var gitLogOutput io.ReadCloser
		if blame == "git" {
			var err error
			gitLogOutput, err = blameworthy.RunGitLog(r.Path, "HEAD")
			if err != nil {
				return err
			}
		} else {
			var err error
			gitLogOutput, err = os.Open(blame)
			if err != nil {
				return err
			}
		}
		gitHistory, err := blameworthy.ParseGitLog(gitLogOutput)
		if err != nil {
			return err
		}
		histories[r.Name] = gitHistory
	}
	return nil
}

func buildBlameData(
	repo config.RepoConfig,
	commitName string,
	path string,
	isDiff bool,
) (string, *BlameData, error) {
	gitHistory, ok := histories[repo.Name]
	if !ok {
		return "", nil, errors.New("Repo not configured for blame")
	}

	fmt.Print("============= ", path, "\n")
	start := time.Now()

	content, err := gitShowCommit(commitName, repo.Path)
	if err != nil {
		return "", nil, errors.New("Commit does not exist")
	}
	showLines := strings.Split(content, "\n")
	commitHash := showLines[0][:blameworthy.HashLength]

	fmt.Print(commitHash, "\n")

	obj := commitHash + ":" + path
	content, err = gitCatBlob(obj, repo.Path)
	if err != nil {
		return "", nil, errors.New("No such file at that commit")
	}

	lines := []BlameLine{}
	var result *blameworthy.BlameResult

	if !isDiff {
		// Easy enough: simply enumerate the lines of the file.

		result, err = gitHistory.FileBlame(commitHash, path)
		if err != nil {
			return "", nil, err
		}

		for i, b := range result.BlameVector {
			f := result.FutureVector[i]
			lines = append(lines, BlameLine{
				orBlank(b.CommitHash),
				b.LineNumber,
				orStillExists(f.CommitHash),
				f.LineNumber,
				i + 1,
				0,
				"",
			})
		}
	} else {
		// More complicated: build a view of the diff by pulling
		// lines, as appropriate, from the previous or next
		// version of the file.

		result, err = gitHistory.DiffBlame(commitHash, path)
		blameVector := result.BlameVector
		futureVector := result.FutureVector

		if err != nil {
			return "", nil, err
		}

		new_lines := splitLines(content)

		old_lines := []string{}
		content_lines := []string{}

		if len(result.PreviousCommitHash) > 0 {
			obj = result.PreviousCommitHash + ":" + path
			content, err = gitCatBlob(obj, repo.Path)
			if err != nil {
				return "", nil, errors.New("Error getting blob")
			}
			old_lines = splitLines(content)
		}

		j := 0
		k := 0

		both := func() {
			lines = append(lines, BlameLine{
				orBlank(blameVector[j].CommitHash),
				blameVector[j].LineNumber,
				orStillExists(futureVector[k].CommitHash),
				futureVector[k].LineNumber,
				j + 1,
				k + 1,
				"",
			})
			content_lines = append(content_lines, old_lines[j])
			j++
			k++

		}
		left := func() {
			lines = append(lines, BlameLine{
				orBlank(blameVector[j].CommitHash),
				blameVector[j].LineNumber,
				//"  (this commit) ",
				blankHash,
				0,
				j + 1,
				0,
				"-",
			})
			content_lines = append(content_lines, old_lines[j])
			j++
		}
		right := func() {
			lines = append(lines, BlameLine{
				//"  (this commit) ",
				blankHash,
				0,
				orStillExists(futureVector[k].CommitHash),
				futureVector[k].LineNumber,
				0,
				k + 1,
				"+",
			})
			content_lines = append(content_lines, new_lines[k])
			k++
		}
		context := func(distance int) {
			// fmt.Print("DISTANCE ", distance, " ",
			// 	til_line, " ", j+1, "\n")
			if distance > 9 {
				for i := 0; i < 3; i++ {
					both()
					distance--
				}
				for ; distance > 3; distance-- {
					j++
					k++
				}
				for i := 0; i < 3; i++ {
					lines = append(lines, BlameLine{
						"        .       ",
						0,
						"        .       ",
						//blankHash,
						0,
						0,
						0,
						"",
					})
					content_lines = append(content_lines, "")
				}
			}
			for ; distance > 0; distance-- {
				both()
			}
		}

		for _, h := range result.Hunks {
			if h.OldLength > 0 {
				context(h.OldStart - (j+1))
				for m := 0; m < h.OldLength; m++ {
					left()
				}
			}
			if h.NewLength > 0 {
				context(h.NewStart - (k+1))
				for m := 0; m < h.NewLength; m++ {
					right()
				}
			}
		}
		end := len(old_lines) + 1
		context(end - (j+1))

		content_lines = append(content_lines, "")
		content = strings.Join(content_lines, "\n")
	}

	elapsed := time.Since(start)
	log.Print("Whole thing took ", elapsed)

	return content, &BlameData{
		result.PreviousCommitHash,
		result.NextCommitHash,
		showLines[1],
		showLines[2],
		showLines[3],
		lines,
	}, nil
}

func gitShowCommit(commitHash string, repoPath string) (string, error) {
	// git show --pretty="%H%n%an <%ae>%n%ci%n%s" --quiet master master:travisdeps.sh
	out, err := exec.Command(
		"git", "-C", repoPath, "show", "--quiet",
		"--pretty=%H%n%an <%ae>%n%ci%n%s", commitHash,
	).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func orBlank(s string) (string) {
	if len(s) > 0 {
		return s
	}
	return blankHash
}

func orStillExists(s string) (string) {
	if len(s) > 0 {
		return s
	}
	return " (still exists) "
}

func splitLines(s string) ([]string) {
	if len(s) > 0 && s[len(s) - 1] == '\n' {
		s = s[:len(s) - 1]
	}
	return strings.Split(s, "\n")
}
