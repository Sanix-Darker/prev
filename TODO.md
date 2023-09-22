## RESOURCES

- https://dev.to/divrhino/adding-flags-to-a-command-line-tool-built-with-go-and-cobra-34f1

## TODO or ROADMAD (choose)

- [ ] add an `optimize` target (get an input code and try to optimize it for you).
    - Taking it from the clipboard.
    - Taking it from a file (you just specify the path and the lineStart/lineEnd of it).
- add an `ai` with :
    - `ls` to list all supported AI APIs (with informations about the price etc...) (also provide websites while listing those)
    - `use` to select the one you want to use (will check if the appropriate selected AI is available) then save the result in a yaml file (~/.config/prev/config.yml) on linux or (....) on MacOs, or (...) on Windows.
    - `show` to see what is used
    - `set-key` you can set an API key only if you have already set what to use as the API AI.

- [ ] set the context-code from any input if any provided (like the clipboard), this should be set and available inside the config of prev.
- [ ] complete the prompt builder
- [ ] add an util that evaluate the difference from two given files
- [ ] add git (.go) integration for checking on branch/commit
- [ ] regarding tests, add fixtures (fixtures/module.py, ...) _like real life code_
    and mock all api service calls + fix all the available tests + add more

- [ ] Add support for these APIs (after checking if they support API calls and what is free/paid):
    - [ ] ChatGPT AI (you can specify the model version) of course (as default it's 3.5)
    - [ ] Bing AI (from Microsoft) (https://www.bing.com/new)
    - [ ] Perplexity AI (https://www.perplexity.ai/)
    - [ ] Google Bard AI (http://bard.google.com/)
    - [ ] Jasper Chat AI (https://jasper.ai/?utm_source=partner&fpr=devinder85)
    - [ ] ChatSonic AI (https://writesonic.com/?via=devinder14)
    - [ ] Claude AI (https://claude.ai/chats)
    - [ ] Llama 2 AI (https://huggingface.co/spaces/ysharma/Explore_llamav2_with_TGI)
    - [ ] HuggingChat AI (https://huggingface.co/chat/)
    - [ ] NeevAI (https://neeva.com/plans)
    - [ ] YouChat (https://you.com/)
    - [ ] Elicit (https://elicit.org/)
    - [ ] Learnt (https://learnt.ai/)
    - [ ] Pi, your personnal AI (https://heypi.com/talk)
    - [ ] Quora Poe AI (https://poe.com/chatgpt)
    - [ ] PREVV.ai Self Hosted  (yes, prev will have one(barely as fast as the others but should be able to respond some interesting stuffs)) + those configurations needed PRE_API_KEY to be set and stored

- [ ] the power of prev should be all the potentials client for that (not just OPEN AI)

- add "typo" correction to the prompt review
- possibility to add prev as a pre-commit tool too
- find a way to track downloads of prev

- [x] fix all .gh-actions error havings (am ignoring this for now)

- [ ] for branch diff, allow these formats branch1:branch2 or if it's only branch provided, check with the master (or the main branch available ? maybe the golang git API have that ? Need to check...)

- [ ] Make it a product paid $1/MONTH and 10$/YEAR
- [ ] write a gihub action that use the CLI to evaluate a Pull request

-> [ ] For a quick access, make a small web app with a form just to get a valid repository and then
it will evaluate the code and make a code review
