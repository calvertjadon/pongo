---
name: next-pongo-ticket
description: Finalize and publish Pongo's next build ticket.
disable-model-invocation: true
---

# Next Pongo ticket

Publish exactly one implementation-ready build ticket from Pongo's staged roadmap. This skill plans the next slice. It does not implement it or publish later slices.

## Sources

Load these before drafting:

- [Choose the staged build-ticket sequence](https://github.com/calvertjadon/pongo/issues/12), including its resolution comments
- the previously integrated build ticket, including candidate evidence and the learner's reflection
- every wayfinding decision linked by the next roadmap entry
- `CONTEXT.md` for game language
- `CODING_STANDARDS.md` for the gate and review contract
- current `main` for existing public behavior and implementation names

GitHub issues are the published artifacts. In prose, wrap every issue link in its title instead of using a bare issue number.

## Process

### 1. Find the next slice

Read the roadmap in order. Select the first build entry without a published ticket whose predecessor has entered `main` and received its post-integration reflection. If the user names an entry, confirm that it is this entry.

**Complete when:** one roadmap entry is identified, every earlier entry is integrated, and no earlier build ticket remains open.

### 2. Fix the review point

Resolve current `main` to a commit SHA. Confirm that the previous ticket's approved commit is an ancestor of it. Use the resolved `main` SHA as the new ticket's review fixed point.

If either check fails, report the exact mismatch and stop before publication.

**Complete when:** the dependency commit is in `main`, and the ticket can name one exact review fixed point.

### 3. Settle the slice

Read the linked decisions, previous reflection, and current code. Preserve settled player behavior and project language. Let current exported names inform the ticket without prescribing internal names the learner should discover.

Find facts in the repository and tracker. Put only unresolved choices to the user. When a player-visible or architectural choice remains open, ask the smallest question that settles it and resume after the answer.

**Complete when:** a learner can implement the slice without guessing about player-visible behavior, state transitions, dependencies, or approval.

### 4. Draft the ticket

Use the roadmap entry's title. Write these sections in order:

1. `## Goal`
2. `## Player-visible behavior`
3. `## Elm Architecture concept`
4. `## Dependencies`
5. `## Exclusions`
6. `## Acceptance criteria`, with stable `AC-1`, `AC-2`, and later identifiers
7. `## Verification`
8. `## Review fixed point`
9. `## Post-integration reflection`

Make each acceptance criterion observable through player behavior, semantic core state, effect requests, or a named boundary. State reset, retention, invalid-event, and boundary behavior where the slice reaches them. Keep tests for plausible bugs in those contracts.

Link settled decisions for their rationale. Put the slice's executable contract in the build ticket so Spec review can judge the branch from that ticket alone.

The verification section must contain the exact automated gate from `CODING_STANDARDS.md` and an interactive TTY procedure for every changed player-visible behavior and its neighboring flow. The review section must name the fixed-point SHA and the exact Standards and Spec inputs.

End with one short reflection that tests the architecture concept introduced by this slice.

**Complete when:** every requirement belongs to this slice, every acceptance criterion has a check, and every exclusion names the next reachable boundary rather than unfinished work inside the slice.

### 5. Check the cut

Compare the draft with the next and previous roadmap entries.

- Keep one player-visible vertical slice.
- Preserve behavior already integrated.
- Leave later behavior in its named later entry.
- Use links instead of copying decision rationale.
- Remove scaffolds, placeholders, speculative abstractions, and acceptance criteria that only inspect implementation details.
- Keep one implementation ticket. Do not decompose it further unless the slice cannot pass its own interactive check in one branch.

**Complete when:** the ticket is independently reviewable, has no hidden prerequisite, and does not implement or specify a later roadmap entry.

### 6. Publish one ticket

Create one GitHub issue with the roadmap title and full body. Apply `enhancement` and `ready-for-human`.

Add the previous build ticket as a native dependency. Its closed state leaves the new ticket unblocked. Add one short progress comment to [Choose the staged build-ticket sequence](https://github.com/calvertjadon/pongo/issues/12) that links the new ticket by title.

Publish no other build issue. Leave the code unchanged.

**Complete when:** one open, unblocked, `ready-for-human` issue exists and the roadmap comment points to it.

### 7. Verify publication

Read the new issue back from GitHub. Check its title, labels, body sections, fixed-point SHA, dependency, and reflection. Query open build tickets and confirm that this issue is the only next learner ticket.

Return the linked ticket title, fixed-point SHA, dependency title, and the learner's next action.

**Complete when:** tracker state matches the draft and the learner can create a branch from the named commit.