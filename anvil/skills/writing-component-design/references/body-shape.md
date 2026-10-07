# Component design body shape

`create` and `validate` reject a body missing any of these headings, in this order:

```
## Does

- <component> owns <responsibility>.
- <component> is the single source of truth for <X>.

## Does not

- <component> does not <boundary that surprised someone or needs emphasis>.
- <component> does not own <Y> - that belongs to <other component>.

## Interfaces

### <operation>
- Input: <args, types>.
- Output: <result shape>.
- Errors: <named failure modes>.
- Pre/postconditions: <what must hold before; what holds after>.
- Black-box: <what a caller can observe, with no internals>.

## Invariants

- <rule that always holds for this component; not restated from the system design>.

## Verification

How a change under this component is proven to work. An issue's `## Verification` predicates are drawn from here.

### Direct
- <in-tree checks: the unit / e2e / regression suites and how to run them>.

### Indirect (live)
- <how to exercise a change through the built/installed/served artifact>.
```

`## Interfaces` is per operation. A component with no consumer interface writes one line saying so.
