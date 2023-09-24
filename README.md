## PREV

A CodeReviewer cli friend in your terminal.
```bash
Get code reviews from AI for any kind of changes (diff, commit, branch).

Usage:
  prev [command]

Available Commands:
  branch      Select a branch from your .git repo(local or remote)
  commit      Select a commit from a .git repo (local or remote)
  completion  Generate the autocompletion script for the specified shell
  diff        review diff between two files changes (not git related).
  help        Help about any command
  optim       optimize any given code or snippet.
  version     Print the application version.

Flags:
  -h, --help   help for prev

Use "prev [command] --help" for more information about a command.
```


```bash
export OPEN_AI=xxxxx
export OPEN_AI_MODEL=gpt-3.5-turbo
```

## REVIEW DIFF FROM TWO FILES

Example of usecase for reviewing the difference between these two files:

- fixtures/test_diff1.py:
```python
def get_bigger_value(given_dict: dict) -> int:

    maxx = 0
    for k in given_dict.keys():
        if given_dict[k] > maxx:
            maxx = given_dict[k]

    return maxx

# this is a comment
# this is a comment

class Test:
    val = 1
```

- fixtures/test_diff2.py:
```python
def get_bigger_value(given_dict: dict) -> int:
    if not given_dict:
        return 0

    return max(given_dict.values())

# this is a comment
# this is a comment

class Test:
    def __init__(self, val) -> None:
        val = val
        val = 1
```

We just need to run prev on those two files and get:
```bash
$ prev diff fixtures/test_diff1.py,fixtures/test_diff2.py
```

```markdown
+     if not given_dict:
-
+         return 0
-     maxx = 0
+
-     for k in given_dict.keys():
+     return max(given_dict.values())
-         if given_dict[k] > maxx:
+
-             maxx = given_dict[k]
+ # this is a comment
-
+ # this is a comment
-     return maxx
+ class Test:
- # this is a comment
+     def __init__(self, val) -> None:
- # this is a comment
+         val = val
-
+         val = 1
- class Test:
+
-     val = 1
fixtures/test_diff1.py
fixtures/test_diff2.py
> chatId: chatcmpl-827FrPAnWLaOeUiu59JOtkt14r4Ya
> responses:
> review:
1. The initial check for an empty dictionary is unnecessary since the subsequent implementation will return 0 if the dictionary is empty.
2. The loop to find the maximum value in the dictionary can be replaced with a single line, `return max(given_dict.values())`.
3. The comments are duplicated and unnecessary, they should be removed.

Suggestion:
```python
def find_max_value(given_dict):
    return max(given_dict.values())

class Test:
    def __init__(self, val):
        self.val = val
```
Note that the `val` assignment in the `Test` class has been modified to use `self.val` to correctly initialize the instance variable.
```
