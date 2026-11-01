# Pongo

Classic Pong in the terminal: a deterministic Core that owns every player-visible rule, driven by a Shell that owns the terminal and host time.

## Language

**Timing sequence**:
One unbroken run of elapsed-time reports for a timed match phase, identified by a token that never repeats. Every countdown and every rally runs on its own; it begins when the Core asks for time and ends when the Core stops asking.
_Avoid_: timer chain, timing generation, timer generation
