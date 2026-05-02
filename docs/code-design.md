# Code Design

Principles, red flags, and rationalizations to watch for when shaping modules and APIs.

## Core Principles

| Principle                  | Rule                                                                |
| -------------------------- | ------------------------------------------------------------------- |
| Deep Modules               | Interface small relative to functionality; resist "classitis"       |
| Information Hiding         | Each module encapsulates design decisions; no knowledge duplication |
| Pull Complexity Down       | Absorb complexity into implementation, not callers                  |
| Define Errors Out          | Redefine semantics so problematic conditions aren't errors          |
| General-Purpose Interfaces | Design for the general case; handle edge conditions internally      |
| Design It Twice            | Consider two radically different approaches before committing       |

## Red Flags

| Red Flag                | Signal                                              |
| ----------------------- | --------------------------------------------------- |
| Shallow Module          | Interface complexity ≈ implementation complexity    |
| Information Leakage     | Same design decision in multiple modules            |
| Repetition              | Near-identical code in multiple places              |
| Special-General Mixture | General mechanism contains use-case-specific code   |
| Vague Name              | Name too broad to convey specific meaning           |
| Hard to Describe        | Can't write a simple comment → revisit the design   |
| Nonobvious Code         | Behaviour requires significant effort to trace      |

## Common Rationalizations

| Rationalization                | Why It's Wrong                                            | What to Do Instead                                                |
| ------------------------------ | --------------------------------------------------------- | ----------------------------------------------------------------- |
| "More classes = better design" | More interfaces to learn, not simpler code                | Apply the complexity test — does splitting reduce cognitive load? |
| "Keep methods under N lines"   | Over-extraction creates pass-throughs and conjoined logic | Extract only when the piece has a meaningful name and concept     |
| "Add a config parameter"       | Pushes complexity to every caller                         | Compute sensible defaults inside the module                       |
| "Structure by execution order" | Causes information leakage across all steps               | Structure by information boundaries instead                       |
| "We might need this later"     | Speculative generality adds interface surface now         | "Somewhat general-purpose" — cover plausible uses only            |
