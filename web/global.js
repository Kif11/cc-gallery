const nextLink = document.querySelector(".next");
const prevLink = document.querySelector(".prev");
const backLink = document.querySelector(".back");

// Set global hotkeys
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
        case "Escape":
        case "s":
            e.preventDefault();
            if (backLink) {
                backLink.click();
            }
            break;
    }
});