package core

type GitChange struct {
	CommitHash   string // Git commit hash
	PreviousFile []byte // Previous version of the file as a byte slice
	FutureFile   []byte // Future version of the file as a byte slice
	Patch        []byte // Patch representing the changes as a byte slice
}

// GetAdditionsAndDeletions takes a repo path and a commit hash and returns the list of additions and deletions for that commit.

// additions, deletions, err := GetAdditionsAndDeletions(repoPath, commitHash)
func GetAdditionsAndDeletions(repoPath, commitHash string) ([]string, error) {
	// r, err := git.PlainOpen(repoPath)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// hash := plumbing.NewHash(commitHash)
	// commitObj, err := r.CommitObject(nil)
	// fmt.Printf("commitObj parents: %v", commitObj.NumParents())
	// if err != nil {
	// 	return nil, nil, err
	// }

	// gitchanges := []GitChange{}

	// // If it's not the initial commit, compare with its parent commit
	// // if commitObj.NumParents() > 0 {
	// parentCommit, err := commitObj.Parent(0)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// // Get the tree of the commit and its parent commit
	// commitTree, err := commitObj.Tree()
	// if err != nil {
	// 	return nil, nil, err
	// }
	// parentTree, err := parentCommit.Tree()
	// if err != nil {
	// 	return nil, nil, err
	// }

	// // Compare the two trees to find additions and deletions
	// changes, err := parentTree.Diff(commitTree)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// for _, change := range changes {
	// 	fmt.Printf("change to name: %v\n", change.To.Name)
	// 	fmt.Printf("change from name: %v\n", change.From.Name)

	// 	before, after, err := change.Files()
	// 	p, _ := change.Patch()
	// 	gitchanges = append(gitchanges, GitChange{
	// 		CommitHash:   commitHash,
	// 		PreviousFile: before,
	// 		FutureFile:   after,
	// 		Patch:        p,
	// 	})
	// }
	// // } else {
	// // 	// This is the initial commit, so we cannot compare with a parent.
	// // 	// We can only list all the files added in this commit.
	// // 	commitTree, err := commitObj.Tree()
	// // 	if err != nil {
	// // 		return nil, nil, err
	// // 	}

	// // 	commitTree.Files().ForEach(func(file *object.File) error {
	// // 		additions = append(additions, file.Name)
	// // 		return nil
	// // 	})
	// // }

	// return gitchanges
	return []string{}, nil
}
