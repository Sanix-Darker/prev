// Package init triggers VCS provider registration via import side-effects.
//
//	import _ "github.com/sanix-darker/prev/internal/vcs/init"
package init

import (
	_ "github.com/sanix-darker/prev/internal/vcs/gitlab"
)
