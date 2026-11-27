# Changelog

## Unreleased

- `Gradient.Linearize()` now verifies every candidate segment at many probe
    points instead of only its midpoint, and no longer discards points based
    on a chord that already drifted from the curve. The measured RGBA error
    now stays under the requested threshold for curved gradients
    (`Smoothstep`, `Basis`, `Catmull-Rom`), and a stricter threshold no longer
    produces a larger error.
- `Gradient.Linearize()` pins the source gradient's own stop positions and
    hard edges, so an exact linear gradient stays exact and discontinuities
    stay confined to a narrow interval.

## v0.11.1

- Fix bug in Lab conversion

## v0.11.0

- New `GradientBuilder.Reset()`
- New `BlendLab`
- `Gradient`'s fields is now public
- `GradientCore` is now public
- New `InterpolationSmoothstep`

## v0.10.2

- Fix bug in `GradientBuilder.Css()`

## v0.10.1

- New `GradientBuilder.Css()`
- Using `uint32` to store preset colors

## v0.10.0

- Added support for transparency
- New `GoColor()`
- Refactor lots of code
- Remove color schemes
