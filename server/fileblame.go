package server

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/livegrep/livegrep/blameworthy"
	"github.com/livegrep/livegrep/server/config"
)

// Blame experiment.

type BlameResult struct {
	CSSClass string
	PreviousCommit string
	NextCommit string
	Lines []BlameLine
}

type BlameLine struct {
	PreviousCommit string
	PreviousLineNumber int
	NextCommit string
	NextLineNumber int
	OldLineNumber int
	NewLineNumber int
	Line string
}

var histories map[string]blameworthy.FileHistory

func InitBlame() (error) {
	git_stdout, err := blameworthy.RunGitLog("/home/brhodes/livegrep")
	if err != nil {
		return err
	}
	histories, err = blameworthy.ParseGitLog(git_stdout)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded commits\n")
	return nil
}

func buildBlameData(
	repo config.RepoConfig,
	commitHash string,
	path string,
) (string, *BlameResult, error) {
	fmt.Print("============= ", path, "\n")
	start := time.Now()
	commits := histories[path]
	i := 0
	for ; i < len(commits); i++ {
		if commits[i].Hash == commitHash {
			break;
		}
	}
	fmt.Print(commitHash, " ", i, "\n")
	if i == len(commits) {
		return "", nil, errors.New("No blame information found")
	}
	blameVector, futureVector := commits.FileBlame(i)
	previousCommit := ""
	if i-1 >= 0 {
		previousCommit = commits[i-1].Hash
	}
	nextCommit := ""
	if i+1 < len(commits) {
		nextCommit = commits[i+1].Hash
	}
	lines := []BlameLine{}
	for i, b := range blameVector {
		f := futureVector[i]
		lines = append(lines, BlameLine{
			b.CommitHash,
			b.LineNumber,
			f.CommitHash,
			f.LineNumber,
			0,
			i + 1,
			"line",
		})
	}
	//fmt.Print(lines, "\n")

	result := BlameResult{
		"",
		previousCommit,
		nextCommit,
		lines,
	}

	// if !ok {
	// 	return "", nil, errors.New("No blame information found")
	// }
	elapsed := time.Since(start)
	log.Print("Whole thing took ", elapsed)

	// data, err := buildFileData(path, repo, commit)
	// if err != nil {
	// 	http.Error(w, "Error reading file", 500)
	// 	return
	// }

	// script_data := &struct {
	// 	RepoInfo config.RepoConfig `json:"repo_info"`
	// 	Commit   string            `json:"commit"`
	// }{repo, commit}

	// body, err := executeTemplate(s.T.FileView, data)
	// if err != nil {
	// 	http.Error(w, err.Error(), 500)
	// 	return
	// }

	obj := commitHash + ":" + path
	fmt.Print("===== ",obj, "\n")

	content, err := gitCatBlob(obj, repo.Path)
	if err != nil {
		return "", nil, errors.New("No such file at that commit")
	}

	return content, &result, nil
}
