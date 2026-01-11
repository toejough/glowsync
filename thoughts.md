I like the timeline. I prefer option A with full names.

I like most of the fields as designed, but one additional principle I'd like to see is consistency of widgets, where
possible. Examples:

- Screen 1 and screen 2 both have source & dest, but screen 1 has them plain and vertical and screen 2 has them in a box
  and horizontal, and the filter dissapears. It would be nice to keep the filter with the source (we're only syncing the
  files that match the filter)
- between screen 3 and 4, source & dest info dissapears entirely. I know we'll start to take up a lot of screen space,
  but let's start with additive consistency and we can talk about collapsing or hiding things when space is constrained
  as a followup. I'd like to keep the source/dest info as boxes on the dashboard.
- also between screens 3 and 4, the activity widget gets bumped down. I'd like to keep it in the same place if possible.
- similarly between screens 4 and 5, the comparison widget dissapears. that's context I'd like to keep around if
  possible.

Other notes:

- I'd like to do source & dest scanning simultaneously if possible, to speed up the comparison step - there's no need to
  serialize those, is there?
- Maybe another design option is to keep the most relevant information at the top - progressively push inactive widgets
  down the list, and introduce new widgets at the top as needed. For example, the final success screen could have the
  following widgets, from top to bottom:
  - Sync Complete! (big font)
  - Progress box (showing 100%)
  - Worker stats
  - Transfer box (everything's done)
  - Comparison box
  - Source & Dest boxes
- would the ongoing activity box fit on the right, alongside the other widgets? We could keep the same design principle
  of most relevant/recent info at the top.

1: answered above (I like option A with full names and I want to make the scanning simultaneous)
2: if we do the activity box on the right, that means we can take as much vertical space as we need for it.
3: modal would be cool.
4: I don't think we need to worry about the no-color fallback right now.
