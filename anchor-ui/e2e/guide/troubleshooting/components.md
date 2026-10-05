# Component test incidents

Real failures found while validating the browser stack. Keep component-runner
repairs here and app-domain incidents in their adjacent guides.

## Storybook scanned a bulk dialog during teardown

The cloud frontend run passed 189 of 190 stories but reported an unnamed
`alert-dialog-action` in the accessibility `afterEach` for
`BulkRunsSequentiallyAndLocksControls`. Its play function stopped as soon as
the completed summary appeared and the second request was recorded. At that
point, the action/progress state had cleared while the closing dialog portal
could still be mounted. The reported button was enabled; the actual blocked
request renders the named, disabled `Working…` action.

The story now captures its alertdialog element before starting, explicitly
asserts the busy action's accessible name and disabled state, retains the
single-in-flight, double-click, progress, cancel/pagination lock and two-call
completion assertions, then waits for that captured element to leave the DOM.
This gives the unchanged axe hook the completed UI instead of an exit frame.
It uses a DOM condition, not a sleep, retry or disabled accessibility rule.

Run the focused Storybook case after any concurrent benchmark finishes:

```sh
pnpm test-storybook src/components/common/datatable/AnchorDataTable.stories.tsx \
  -t "Bulk Runs Sequentially And Locks Controls"
```

Then run the full Storybook suite and inspect the cloud frontend check. A green
result is required before treating this synchronization repair as verified.

## Storybook did not observe a schema parse error after untargeted paste

After the bulk-dialog repair, cloud CI passed that story but failed
`LicenseSchemaFormDialog > Unreadable Source Blocks Submit`. Immediately after
pasting invalid DSL, `getByText` could not find the error. Changing it to
`findByText` passed three local focused runs and the complete 190-story suite,
but the next cloud run still failed after waiting for the error. Waiting alone
was therefore insufficient.

The installed Storybook `userEvent.paste(text)` implementation dispatches to
`document.activeElement`; a supplied string uses a synthetic data transfer,
so this is not evidence of an operating-system clipboard race. Dialog focus
could change that target, but those cloud DOM captures were truncated before
the textbox and did not capture its value. No API or network cause was
reproduced.

Replacing paste with `userEvent.type(editor, invalidSource)` added an assertion
of the textbox's complete value before checking the error. The new create dialog
serializes its blank field to an empty source, which is also asserted before
typing. That change passed locally and in one cloud run, but the following cloud
run proved the input was incomplete: it expected the full DSL and received only
`m`. The parse-error and disabled-submit assertions were not reached.

The app's editor and textarea are module-level components without an
input-dependent key, conditional textarea replacement or source-reset effect.
The installed Base UI focus manager queues its default initial focus through a
microtask and the next animation frame. Its queued callback can still focus
the first tabbable control when it recorded focus already inside the dialog.
That scheduling is confirmed in source, but waiting for its initial focus also
passed one cloud run and failed the next at the same commit with only `m` in
Fields. It did not fix the incomplete input. The exact focus-loss cause remains
unproven; do not change the product based on this test-runner evidence alone.

This story verifies invalid-source validation, so it uses the native Vitest
browser `userEvent.fill(editor, invalidSource)` API. It targets the real Fields
element through the Playwright provider and triggers the input change without
a sequence of keyboard actions that depend on global focus. The
[official fill documentation](https://vitest.dev/api/browser/interactivity.html#userevent-fill)
recommends it when individual keypress behavior is not the subject of the test.
The story still checks Text mode, an empty initial source, unchanged textbox
identity, the complete entered value, the exact parse error and disabled submit.

Vitest 4.1.10 sets `globalThis.__vitest_browser__` in
`@vitest/browser/dist/state.js`; its `vitest/browser` entry throws outside Browser
Mode. Guard the lazy import by the presence of that runner global so ordinary
interactive Storybook can keep using `storybook/test` input actions. Both
environments execute the same assertions; there is no fallback after an input
or assertion failure. See the
[Vitest browser context entry](https://github.com/vitest-dev/vitest/blob/v4.1.10/packages/vitest/browser/context.js)
for the import boundary. Verify the focused story three times without retries,
then run the complete Storybook suite and repeat the cloud frontend check at
the same commit:

```sh
pnpm test-storybook src/components/license/LicenseSchemaFormDialog.stories.tsx \
  -t "Unreadable Source Blocks Submit" --retry 0
```

