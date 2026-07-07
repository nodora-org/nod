# nod

![GitHub last commit (branch)](https://img.shields.io/github/last-commit/nodora-org/nod)
![GitHub License](https://img.shields.io/github/license/nodora-org/nod)
![GitHub Tag](https://img.shields.io/github/v/tag/nodora-org/nod)

**nod** is an agent governor. You give it a proposed action as JSON and a policy written as
rules; it answers `allow`, `ask`, or `deny`, and shows which rule decided and
why. The rules are pure functions from input to verdict, so nothing ties them to
a specific domain.

Its main use is governing what an AI agent may do. A `hook` adapter converts an
agent's tool call (a shell command, a file write, a network fetch) into that
JSON, runs it through the policy, and returns a decision for the agent's host to
enforce. [Claude Code](https://code.claude.com) works today via its `PreToolUse`
hook; supporting another agent means writing another adapter.

Rules live in `.ruleset` files. Add or edit them to change the policy, and nod
picks up the change on the next call.

Policy comes from two places, and both are just directories of `.ruleset` files:

- **Global policy** lives in `$NOD_HOME/rulesets/` (`$NOD_HOME` defaults to
  `$HOME/.nod`). These are your own rules that apply everywhere. nod reads this
  directory by default; `-d <dir>` overrides it.
- **Local policy** lives in a `.nod/` folder at the root of the project being
  worked in. nod discovers it automatically and layers it on top of the global
  policy. Because it can arrive with a cloned project, it is trusted less: it may
  tighten the policy but not loosen it (see [Local policy](#local-policy-a-nod-folder)).

## Install

```sh
go install nodora.org/nod@latest
```

Or build from source:

```sh
go build -o nod .
```

Put the `nod` binary on your `PATH`; the commands below assume that.

## Writing rules

Input reaches nod as a JSON object. An agent tool call looks like
`{"tool":"bash","cmd":"rm -rf build"}`.

Rules are written in [Nodora](https://nodora.org), a small declarative rule
language. Each rule is a pure function from the input to a set of emitted
_signals_, type-checked when it compiles, with no I/O and no loops. A policy rule
emits one of four signals:

| signal          | meaning                                 |
| --------------- | --------------------------------------- |
| `Deny(reason)`  | block the call                          |
| `Ask(reason)`   | ask the user first                      |
| `Allow(reason)` | permit the call                         |
| `Log(message)`  | record a note, no effect on the verdict |

Those four are the only signals nod acts on. A rule can emit some other signal,
but nod has no handler for it, so the emit does nothing: a rule can never grant a
capability nod does not already expose.

nod loads every `.ruleset` file in the policy directory, runs the input through
all of them, and merges the signals they emit into a single verdict.

## The merge: deny-wins

| If any rule emits… | verdict                         |
| ------------------ | ------------------------------- |
| `Deny`             | **DENY** (cannot be overridden) |
| else `Ask`         | **ASK**                         |
| else `Allow`       | **ALLOW**                       |
| else (nothing)     | the default                     |

`Deny` outranks the others, so adding a rule file can only ever make the policy
stricter for a call that some other rule denies. One case to know: an `Allow` can
promote a call that nothing else has an opinion on, so a rule file that emits
`Allow` does widen what gets through when no rule denies. For local rules nod
handles this by suppressing their `Allow` (see below); for your own global rules
it is up to you.

The default for a call no rule matched depends on the command: `nod decide` defaults
to `deny`, while `nod hook` defaults to `defer`, which hands the call back to
Claude Code's normal permission flow instead of blocking everyday tools.

## Commands

Decide a single input and print the verdict with its provenance:

```sh
echo '{"tool":"bash","cmd":"rm -rf build"}' > call.json
nod decide -i call.json
```

With no `-d`, `nod decide` reads the global policy from `$NOD_HOME/rulesets`.

For scripting, `-q` (`--quiet`) prints just the verdict word (`ALLOW`/`ASK`/`DENY`)
with no provenance, and `--fail-on <ask|deny>` makes `decide` exit `1` when the
verdict is that strict or stricter, so a call can gate a command:

```sh
if nod decide -i call.json -q --fail-on deny; then
  run_the_tool   # verdict was allow or ask, and nod ran cleanly
else
  echo blocked   # denied, or nod itself errored; either way, don't proceed
fi
```

Any non-zero exit means "do not proceed": a policy block and a failed run both
land there, which is the fail-closed behavior you want for a gate. The verdict
word on stdout still tells the two apart if a script needs to.

Compile-check every ruleset, reporting each file as ok or with its compile error
and exiting non-zero if any fail. This reads no input and makes no decision, so it
fits a pre-commit hook or a CI step that catches a broken rule before it ships:

```sh
nod check
```

## Use with Claude Code (PreToolUse)

`nod hook` adapts the policy to Claude Code's `PreToolUse` hook. Claude sends each
tool call as JSON on stdin before running it, and nod prints a decision:

```sh
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf build/"}}' | nod hook --adapter claude
# {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",
#   "permissionDecisionReason":"danger.ruleset:BlockRmRf: recursive force remove"}}
```

It translates Claude's `(tool_name, tool_input)` into the shape the rules read:
`Bash` becomes `{tool:bash,cmd}`, `Write` and `Edit` become `{tool:…,path}`,
`WebFetch` becomes `{tool:webfetch,url,host}`, and so on. Verdicts map directly:
`Deny` to `"deny"`, `Ask` to `"ask"`, `Allow` to `"allow"`, and a call
no rule matched to `"defer"`, which lets Claude's own permission flow decide.

Because unmatched calls defer, the hook only intervenes on calls a rule actually
addresses. Everyday tools pass through to Claude untouched.

To turn it on, add a hook to the `.claude/settings.json` of the project you want
governed:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "nod hook --adapter claude"
          }
        ]
      }
    ]
  }
}
```

With no flags, `nod hook` reads its global policy from `$NOD_HOME/rulesets`,
applies any `.nod/` folder in the governed project, and appends decisions to
`$NOD_HOME/audit.jsonl`. Pass `-d <dir>` to use a different global policy dir,
`-log <path>` for a different log, or `-no-log` to turn logging off. The `--adapter`
flag selects the agent protocol; `claude` is the only one currently supported.

Each call runs a fresh `nod hook`, so editing a rule file takes effect on the
next tool call without restarting anything.

## Local policy: a `.nod/` folder

A project can carry its own rules in a `.nod/` directory at its root:

```
your-project/
  .nod/
    deny-migrations.ruleset      # this project forbids editing generated migrations
    require-review-on-infra.ruleset
  src/
  ...
```

`nod` finds it on its own. You don't pass `--local` for the common case:
`decide` and `check` discover a `.nod/` by walking up from the working directory,
so a call from a subdirectory still finds the project root. `hook` does the same,
except its adapter picks where the search starts from the agent's own signal (the
Claude adapter starts at `$CLAUDE_PROJECT_DIR`, falling back to the tool call's
cwd). The folder layers on top of the global policy; there is nothing to
configure per project. Pass `--local <path>` to use a specific directory instead,
or an empty one to ignore a nearby `.nod/`.

Rules from a `.nod/` folder are treated as lower trust, because the folder ships
inside the project and may come from a clone you do not control. The local layer is
loaded **tighten-only**: its `Deny` and `Ask` count, but any `Allow` it emits is
suppressed. A project can make the policy stricter for its own needs, but it
cannot auto-approve a call to get past the global policy or the user.

If you do trust a project's rules, for instance your own monorepo, pass `--trust`
to let its `Allow` count. Pass `--local` to try a specific `.nod/` against a call:

```sh
nod decide -i call.json --local ../your-project/.nod            # tighten-only
nod decide -i call.json --local ../your-project/.nod --trust    # trust its Allow
```

## The audit log

`nod hook` appends one JSON line per decision to `$NOD_HOME/audit.jsonl` by
default. It is a record of what the policy decided and why. Use `-log <path>` to
change the location or `-no-log` to disable it.

Because evaluation is pure, the same input and the same rules always produce the
same verdict, so this log doubles as a regression corpus: replay it against an
edited rule set and diff the verdicts to see exactly which past decisions a change
would flip before you adopt it.

## What nod is and is not

nod is the decision layer. It answers "may this call run?" for one call at a time.
It has no state, no memory, and no loop of its own. The agent loop, the actual
tool execution, and the handlers behind each signal all live in the host that
calls nod.

You write the rules. nod does not generate or modify its own policy. Rules are
type-checked when they compile, sandboxed at runtime, and limited to the fixed
signal vocabulary, which keeps rule-authoring cheap and safe to automate later.
But a component that has an agent propose, validate, and adopt new rules is not
included here.

## License

Licensed under the [Apache License 2.0](./LICENSE).