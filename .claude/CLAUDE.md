# Claude Memory File

## Working Guidelines

never unstage or undo my changes (unless explicitly asked), understand and incorporate them!

This project contains the source code for the go language and it's compiler and its toolchains.
It is very comprehensive and we need to be careful when searching for a code to not use up all our clout code subscription context window and usage.

Do NOT explore the whole codebase structure and architecture

We are trying to achieve small modifications to the compiler only and nothing else thus we only want to recompile the compiler and not the whole toolchain.

Always start simple and when the simple case works add more complexity. 

Stick to one suggested approach first, NO "Actually, let me take a different approach." until we have tried out the first one.
If one approach fails, note it in ./probes/TRIED.md and try another approach.
After one feature is done you can delete this file.


!! Important! Never do destructive commands like remove, rm, git clean, etc even in YOLO mode: get explicit user confirmation !!

After long tasks with surprising observations, 
Write down the most essential insights of what you’ve learned into a file starting with that topic ending in -guide.md 
Then, add a line in CLAUDE.MD noting that this guide can be found under that filename.”




# Testing Guidelines
ALWAYS RUN tests before doing any changes!
To check the current status and that everthing is (should be) OK.

NEVER modify existing tests!
INSTEAD of touching tests in goo, create new tests in the probes folder
Do not remove tests that are not passing instead leave them there for later fixing.

Do not re-create tests for what is already working, if this is already tested elsewhere.

after performing changes recompile bin/go before testing!

Only report victories if the original complaining test is passing, check again after the changes!

NEVER modify my tests in goo folder! Only add one new test after I've approved the fully functional major feature otherwise leave your tests in probes

# Consolidation
When failing to make progress don't celebrate (prior progress), 
Do NOT repeat What Works well from previous progress,
just be candid and leave the failing tests in the probe directory and summarize the difficulties that have been encountered and possible (alternative) ways forward.

Only create ONE new test per feature and reuse existing tests for very similar features.

# Test Writing
Usually when you create one test there's no more need to modify it unless you really missed something
USE existing tests instead of writing new ones
Before declaring a bug/task as fixed run the test again. If it fails report it with ❌


To avoid code duplication do a quick git history search (grep?) to see if there have been related changes

Ignore TODO.md, it's only for myself   add to claudeignore and remove this line



## Debugging
• don't Temporarily disable test functionality that worked before, instead push forward to resolve the issue
 

With the visitor pattern, we can only replace nodes in place, so not always applicable? 
we sometimes still need to use the verbose walker for rare cases where we need to replace a node with a completely different node.
Or use a hybrid.

## Transformer Architecture
The new NodeTransformer interface eliminates code duplication by using a centralized visitor pattern. The transformer architecture guide can be found under transformer-architecture-guide.md in the transforms folder. Key principle: Central visitor does ALL traversal, transformers handle only their specific node types (2-3 lines of CanHandle logic instead of 50+ lines of traversal duplication).


# Assume mishearings
I am using speech recognition which might misunderstand some words:
parcel => parser , fights => files

# automatic imports
since we moved the import resolver in the pipeline to after the Transformers all the import issues were gone
If you ever encounter automatic import problems again just look at: 
goo/test_strings_auto_import.goo                                                                   │
goo/test_list_methods.goo
(together with their implementation and maybe their commits)

# Hard extensions
When creating new tokens or new expression types they need to be registered in the compiler visitor and Walker so that they are treated like normal nodes


# Netlify Deployment
For deploying web apps to Netlify, see the complete guide in Netlify-deploy.md which documents the working osascript-based approach and lists all failed methods to avoid.

Subagents, plugins, skills, hooks, mcp scripts+servers, capabilities, connectors, tasks, apps, custom-gpts, ... the space needs some serious consolidation and standardization!
