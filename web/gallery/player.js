const videoEl = document.querySelector("video");
const nextLink = document.querySelector(".nav-next");
const prevLink = document.querySelector(".nav-prev");

// Set player hotkeys
document.addEventListener("keydown", (e) => {
    switch (e.key) {
        case "ArrowLeft":
            case "a":
                if (prevLink) {
                    prevLink.click();
                }
                break;
            case "ArrowRight":
            case "d":
                if (nextLink) {
                    nextLink.click();
                }
                break;
        case "m":
            videoEl.muted = !videoEl.muted;
            break;
    }
});