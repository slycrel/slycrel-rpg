# Drawing an icon with a local model

How `cmd/pixelsmith` works, what it is for, and the failure modes that are worth
knowing before you spend an afternoon rediscovering them.

It exists for one situation: **the game needs a picture the bundle does not
contain.** That has happened exactly once so far — polearms, where six sources
and a composite attempt all failed (the search is written up in
[PLAN.md](PLAN.md)'s Phase 5). It is not a substitute for looking. Looking is
cheaper, produces better art, and the whole asset pass is a record of the
bundle turning out to have the thing after all.

## The short version

```bash
# 1. draft. -head means "draw only the business end; I will lay the handle"
go run ./cmd/pixelsmith gen -name spear -head 6 -n 12 -model devstral:latest \
  -desc "a spearhead lying along the diagonal, in line with its haft: ..."

# 2. look at shots/pixelsmith-spear/sheet.png, best-first

# 3. keep one
go run ./cmd/pixelsmith adopt -name spear -pick 5

# 4. render it, band it, wire it
go run ./cmd/assetpipe build
```

`adopt` writes `data/art/<name>.txt`. That file is the deliverable — sixteen
lines of palette indices — and `assetpipe drawn` renders it into the weapon set
where `bands` and `manifest` pick it up like any cut icon.

## Why a language model rather than an image model

The target is sixteen pixels square on a six-colour palette. Diffusion does not
draw that. It draws a thousand pixels of something pixel-art-*shaped* and leaves
you to reduce it, and this project already has the counter-example on file: the
ability icon set is painted 128px art, and squeezed into a menu row's 16px box
every weapon in the shop was a purple smudge.

A language model can emit the grid itself — right size, right palette, no
resampling, every cell inspectable before it is accepted. The task is 256 cells
choosing between seven symbols. That is small enough to be tractable and small
enough to review honestly.

## What is committed, and why it is a grid

`data/art/spear.txt` holds indices, not colours, and no PNG is committed at all.
Three reasons, all load-bearing:

- **`assetpipe build` must stay byte-reproducible.** A model is not
  deterministic; rendering a fixed grid is. Running a model inside the pipeline
  would mean the manifest changed every time anybody built it. Verified by
  deleting `assets-raw/_generated` twice and hashing the result.
- **The repository stays art-free.** The grid says which of six palette slots
  each cell uses. The palette itself is read out of the extracted pack at build
  time by `internal/pixelpal`. Nothing purchased is in git.
- **It is reviewable.** A diff shows that a pixel moved.

## The five things that went wrong

In the order they were hit, because each one only became visible after the
previous was fixed.

**1. Showing the real icons as examples teaches copying, not style.** Seeded
with the weapon grids, `qwen3:30b` handed `sword1` back almost cell for cell,
twice, and `devstral` spent a batch redrawing `staff3` with the head moved.
Useless as a spear — and worse than useless as an artefact, because a near-copy
of a purchased sprite *is* the purchased sprite and must not be committed.

**2. Removing the examples removes the ability.** With the house style described
in prose instead — measured off the real icons and stated as numbers, so it
cannot drift from what it claims to describe — the same models produced
unstructured blobs. The grids had been doing real work.

**3. A constructed example is the way out.** `taperExample` draws a plain
geometric taper: a triangle the tool generates, which is nobody's art and can be
shown freely. It carries the notation and the idea of narrowing without giving
the model anything worth reproducing. Heads went from knobs to recognisable
points on the run it was added.

**4. The example has to be drawn on the set's own axis.** The first one was a
vertical triangle, and an axis-aligned leaf pasted onto a diagonal haft reads as
a flag on a pole. Everything in this pack is drawn lower-left to upper-right;
the example is now a taper along that diagonal.

**5. A filter written for the wrong axis rejects everything.** `headScore`
originally demanded "taller than wide" — true of a vertical leaf, false of a
diagonal blade, whose bounding box is square. The moment the prompt started
asking for a diagonal point, the filter discarded every reply and the run failed
with "no usable grids". It measures along the anti-diagonal now.

## Decomposition is what actually made it work

A weapon icon is mostly a straight line, and **a straight line is the part a
program draws better than a model** — perfectly even, on the exact diagonal the
set uses, every time. Asking for the whole icon spends a very limited spatial
budget on the easy nine tenths.

So `-head N` asks the model for an N×N head only, and the tool lays the haft.
Every candidate after that change was a connected, correctly-proportioned hafted
weapon. Two details matter:

- **The join is found, not promised.** Telling the model "the haft will meet
  your bottom-left corner" does not make it draw there; the first version left a
  five-column gap, which renders as a stick and a separate lump. The tool now
  locates the lowest-leftmost cell the model actually inked and runs the haft up
  to it.
- **The haft's shading is copied, not invented.** It laid the shadow cell below
  the body cell; `mace1` and `pick1` both put it to the right on the same row.
  One pixel per step, and it is the difference between a stick and a haft —
  invisible alone, obvious in a row of eight.

## Models

| model | verdict |
|---|---|
| `gpt-oss:20b` | best heads of the four. Far too slow to iterate with — use it to confirm a shape, not to search for one. If it fails to load with a tensor overflow the local blob is corrupt: `ollama rm` and re-pull, which is not an ollama version problem. |
| `devstral:latest` | fast enough to iterate. Does the searching. |
| `qwen3:30b` | the worst copier of the four, and it leaks its reasoning into the reply. Size did not help here; decomposition did. |
| `qwen2.5-coder:3b` | produces noise on a spatial task. |

## Rules for the next one

- **Look for the picture first.** Every one of the eleven passes before this
  found the art already in the bundle. This tool is what is left after that
  fails.
- **Judge it on the shelf, not alone.** The haft-shading bug was invisible in
  isolation and obvious in a row of eight. Render the whole set.
- **If the filter rejects everything, suspect the filter.** It is describing a
  shape, and a shape description written for one orientation is a shape
  description that is wrong for another.
- **Do not hand the model the pack's art.** Style transfers through
  measurements and a constructed example. Pixels transfer through copying.

## Licence

The palette and the style measurements come from `pixelartminingcrafting`,
Tier B in [ASSET-LICENSING.md](ASSET-LICENSING.md), whose terms permit
derivative works. **Mana Seed is kept out of this entirely and deliberately** —
its licence forbids use in an AI project, which is why the seeds are drawn only
from the crafting pack's weapons.
