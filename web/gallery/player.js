const videoEl = document.querySelector("video");

// Set player hotkeys
document.addEventListener("keydown", (e) => {
    switch (e.key) {
        case "m":
            videoEl.muted = !videoEl.muted;
            break;
    }
});