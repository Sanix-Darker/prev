## RESOURCES

- https://dev.to/divrhino/adding-flags-to-a-command-line-tool-built-with-go-and-cobra-34f1

## TODO or ROADMAD (choose)

- implement the streaming mode of prev
    - for chatGPT..

- add `config` command to set the configuration file of prev.
- use ../prev-ai with LLMA and do real-life tests on it.
- [ ] add loaders/spinners when the reqeust have been made
- [ ] use prompt-ui pour des process en multi steps
- [  ] add an `ai` with :
    - `ls` to list all supported AI APIs (with informations about the price etc...)
        (also provide websites while listing those)
    - `use` to select the one you want to use (will check if the appropriate selected AI is available)
        then save the result in a yaml file (~/.config/prev/config.yml) on linux or (....) on MacOs, or (...) on Windows.
    - `show` to see what is used
    - (edit: should be a part of config) `set-key` you can set an API key only if you have already set what to use as the API AI.
- pul all prompts inside a seperated file.
- prev should be like : https://github.com/TheR1D/shell_gpt but better
- provide a docker command to run prev too with given envs...
<!-- docker run --rm \ -->
<!--        --env OPENAI_API_KEY="your OPENAI API key" \ -->
<!--        --volume gpt-cache:/tmp/shell_gpt \ -->
<!-- ghcr.io/ther1d/shell_gpt --chat rainbow "what are the colors of a rainbow" -->
- [ ] A GIGA CONCURANT : (https://github.com/appleboy/CodeGPT)
- [ ] on code review mode for (branch or commit, extract also commit title, message, list of files changes)
- [ ] Another cool example (of github integration with GPT and AI): (https://github.com/sweepai/sweep)
- [ ] Pipe the output of the review inside the glow CLI (like,
        add the possibility to add in the configuration a markdown renderer)
- [ ] Maybe cover most of the use case here :https://www.youtube.com/watch?v=7k0KZsheLP4 (but for the review-code idea)
- [ ] add tests
- [ ] define a max item diff to print to the screen and just make a ...
- [ ] but while reviewing, press n (next) for each set of changes in case of a branch/commit review

- [ ] set the context-code from any input if any provided (like the clipboard), this should be set and available inside the config of prev.
- [ ] complete the prompt builder
- [ ] add an util that evaluate the difference from two given files
- [ ] add git (.go) integration for checking on branch/commit
- [ ] regarding tests, add fixtures (fixtures/module.py, ...) _like real life code_
    and mock all api service calls + fix all the available tests + add more

- [  ] Find a way to reuse a precedent chatId to keepthe context of a previous
  chat
- [ ] Add support for these APIs (after checking if they support API calls and what is free/paid):
    - [x] ChatGPT AI (you can specify the model version) of course (as default it's 3.5)
    - [ ] Bing AI (from Microsoft) (https://www.bing.com/new) (https://learn.microsoft.com/en-GB/azure/ai-services/openai/overview)
    - [ ] Perplexity AI (https://www.perplexity.ai/) (https://github.com/nathanrchn/perplexityai/blob/main/Perplexity.py)
    - [ ] Google Bard AI (http://bard.google.com/) (waiting list)
    - [ ] Jasper Chat AI (https://jasper.ai/?utm_source=partner&fpr=devinder85) (no go ??)
    - [ ] Claude AI (https://claude.ai/chats) (https://docs.anthropic.com/claude/reference/client-sdks)
    - [ ] Llama 2 AI (https://huggingface.co/spaces/ysharma/Explore_llamav2_with_TGI)
    - [ ] HuggingChat AI (https://huggingface.co/chat/)
    - [ ] NeevAI (https://neeva.com/plans)
    - [ ] YouChat (https://you.com/)
    - [ ] Elicit (https://elicit.org/)
    - [ ] Learnt (https://learnt.ai/)
    - [ ] Pi, your personnal AI (https://heypi.com/talk)
    - [ ] Quora Poe AI (https://poe.com/chatgpt)
    - [ ] PREVV.ai Self Hosted  (yes, prev will have one(barely as fast as the others but should be able to respond some interesting stuffs)) + those configurations needed PRE_API_KEY to be set and stored
        - https://github.com/LinkSoul-AI/Chinese-Llama-2-7b
        - https://github.com/soulteary/docker-llama2-chat
        - https://github.com/soulteary/llama-docker-playground
        - https://github.com/getumbrel/llama-gpt
        - https://github.com/soulteary/docker-llama2-chat/tree/main

- [ ] the power of prev should be all the potentials client for that (not just OPEN AI)
- add "typo" correction to the prompt review
- possibility to add prev as a pre-commit tool too
- find a way to track downloads of prev

- [ ] for branch diff, allow these formats branch1:branch2 or if it's only branch provided, check with the master (or the main branch available ? maybe the golang git API have that ? Need to check...).

- [ ] Make it a product paid $1/MONTH and 10$/YEAR (not sure for this).
- [ ] write a gihub action that use the CLI to evaluate a Pull request.
- [ ] write a nvim plugin that use the CLI to evaluate code input and give
  suggestions about it.

-> [ ] For a quick access, make a small web app with a form just to get a valid repository and then
it will evaluate the code and make a code review
