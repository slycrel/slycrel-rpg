# The test suite, on a machine with no screen.
#
# Ebitengine will not initialise without a monitor. On macOS that is not a
# figure of speech: internal/ui's package init calls newUserInterface, which
# enumerates monitors, and with none it either segfaults (darwin's
# currentMouseLocation dereferences the nil its own caller is written to
# expect) or, with that bug fixed, panics with "no monitor was found". Either
# way every test in internal/game fails, because importing the package is
# enough. There is no headless mode and no environment variable for it.
#
# So the answer is not to go without a display, it is to use one nobody is
# looking at. Linux has Xvfb; macOS has nothing equivalent, which is why this
# is a container rather than a script. Mesa's llvmpipe supplies the GL that
# Xvfb's X server does not.
FROM golang:1.26-bookworm

# What Ebitengine links against on Linux, plus the X server it will talk to.
# libasound2-dev is for the audio package: no test opens a device, but the
# package has to build.
RUN apt-get update && apt-get install -y --no-install-recommends \
        xvfb xauth \
        libgl1-mesa-dev libglu1-mesa-dev mesa-utils libgl1-mesa-dri \
        libx11-dev libxcursor-dev libxi-dev libxinerama-dev \
        libxrandr-dev libxxf86vm-dev \
        libasound2-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

# A sound card that goes nowhere. The demo tour and the audit both build a real
# audio bank, and oto opens ALSA's "default" device when it does — in a
# container there is no default, and the game exits on an audio error before it
# has drawn anything. A null PCM is the honest answer: the tour runs silent by
# design anyway, and the audit is checking that files decode, not that they can
# be heard.
RUN printf 'pcm.!default { type null }\nctl.!default { type null }\n' > /etc/asound.conf

# llvmpipe rather than whatever the host might have offered. A container has no
# GPU and asking for one produces a driver error rather than a fallback.
ENV LIBGL_ALWAYS_SOFTWARE=1
ENV GALLIUM_DRIVER=llvmpipe

# Xvfb by hand rather than through xvfb-run.
#
# xvfb-run hung: the X server came up, the command never ran, and the container
# sat at nought per cent for a quarter of an hour with an empty log. Whatever
# its wait-for-ready loop was waiting for, it was not arriving in a container
# with no /tmp state and no session. Starting the server and watching for its
# own socket is four lines and cannot hang for a reason nobody can see.
RUN printf '%s\n' \
    '#!/bin/sh' \
    'set -e' \
    'Xvfb :99 -screen 0 1280x800x24 -nolisten tcp &' \
    'xvfb=$!' \
    'i=0; while [ ! -e /tmp/.X11-unix/X99 ] && [ $i -lt 100 ]; do i=$((i+1)); sleep 0.1; done' \
    '[ -e /tmp/.X11-unix/X99 ] || { echo "headless: Xvfb never came up" >&2; exit 1; }' \
    'export DISPLAY=:99' \
    '"$@"; status=$?' \
    'kill $xvfb 2>/dev/null || true' \
    'exit $status' \
    > /usr/local/bin/with-display && chmod +x /usr/local/bin/with-display

WORKDIR /src
ENTRYPOINT ["with-display"]
CMD ["go", "test", "./internal/..."]
