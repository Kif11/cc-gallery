const backLink = document.querySelector(".nav-back");

// Set global hotkeys
document.addEventListener("keydown", (e) => {
    switch (e.key) {
        case "Escape":
        case "ArrowUp":
        case "w":
            e.preventDefault();
            if (backLink) {
                backLink.click();
            }
            break;
    }
});