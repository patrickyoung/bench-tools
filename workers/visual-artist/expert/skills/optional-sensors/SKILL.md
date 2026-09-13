---
name: optional-sensors
description: Use only when a brief meaningfully calls for camera, microphone, location, or orientation; add consent-based local processing and a manual alternative.
---

# Optional sensor interactions

Use this skill only when a sensor materially supports the artistic intent. Pointer, touch, keyboard, or a native manual control is often better.

For each sensor, document its purpose and an attractive manual fallback. Never request access during import, mount, hover, scroll, or generic navigation. Provide a separately labeled enable control, a plain-language local-use explanation before permission, visible pending/active/denied/unavailable/ended status, and stop.

- Feature-detect secure context and APIs. Orientation permission is platform-specific; do not promise universal support.
- Keep a separate generation token per input. Starting a microphone must not invalidate a camera request. Stop-all and destroy invalidate every input. Guard each asynchronous continuation, including video play and audio resume; release resources returned to obsolete requests without stopping a newer request's resources or changing its status.
- Camera/microphone: process locally, never record/upload/persist by default, stop every media track, and handle track-ended events.
- Microphone: obey Web Audio user-gesture rules; disconnect nodes and close/suspend the context as appropriate.
- Location: request only purposeful precision, avoid retaining coordinates, clear every watch, and offer user-selected place/coarse manual input.
- Orientation: request platform permission only from its enable action and provide sliders/keyboard controls.
- Stop resources on explicit stop, destroy, and pagehide. Remove all listeners and callbacks.
- Do not identify people, recognize faces, infer biometrics, or derive/persist personal attributes.

Automated development uses injected browser API mocks and synthetic events only. Test granted, denied, unavailable, pending-then-stopped, source-ended, repeated start/stop, destroy, and pagehide paths. State explicitly that real hardware was not activated or tested unless an authorized human test actually occurred.
