const getFileName = (path) => path.split("/").slice(-1).pop()

// Restore scroll position from URL parameter
function scrollMediaIntoView() {
    const curMedia = new URLSearchParams(window.location.search).get('p')

    if (!curMedia) {
        return
    }

    const allMedia = document.querySelectorAll(".lazy");

    allMedia.forEach((m) => {
        const mediaName = getFileName(m.dataset.url)

        if (mediaName !== curMedia) {
            return
        }

        m.scrollIntoView({
            behavior: "instant",
            blocl: "center",
            inline: 'center'
        })
    })
}

// We keep track of current media that need to be scrolled into view via URL "p" paramenter
// this function updates it as user scroll the gallery
function updateCurrentMediaInURL(visibleElements) {
    if (visibleElements.size < 2) {
        return
    }

    // Pick middle media elemnt from visible media on the screen
    const middle = Array.from(visibleElements)[Math.floor(visibleElements.size / 2)];
    const mediaName = getFileName(middle.dataset.url)

    // Make new URL query parameters
    const url = new URL(window.location.href);
    url.searchParams.set('p', mediaName);

    // Update URL without reloading the page
    window.history.replaceState(null, '', url.toString());
}

// Update filter cookie and input value
function updateFilter() {
    const searchParams = new URLSearchParams(window.location.search);
    const queryFilter = searchParams.get('filter')

    if (queryFilter) {
        // If query parameter provided set filter value and cookie
        setCookie("filter", queryFilter);
        input.value = queryFilter;
    } else {
        // Set filter from cookies value
        const filter = getCookie("filter")
        if (filter) {
            input.value = filter;
        }
    }
}

function loadVisibleElements(els) {
    els.forEach(el => {
        if (el.hasAttribute("src")) {
            return;
        }
        el.setAttribute("src", el.dataset.url);
    });
}

// Watch for media withing viepor and load it as it come to view
function lazyLoadMedia() {
    const lazyMediaEls = document.querySelectorAll(".lazy");
    let scrollTimeout;
    let visibleElements = new Set();

    let mediaLoadObs = new IntersectionObserver(function (entries, observer) {
        entries.forEach((entry) => {
            if (entry.isIntersecting) {
                visibleElements.add(entry.target);
            } else {
                visibleElements.delete(entry.target);
            }
        });
    });

    lazyMediaEls.forEach((el) => {
        mediaLoadObs.observe(el);
    });

    // Load images and update URL when scrolling stops
    window.addEventListener('scroll', () => {
        clearTimeout(scrollTimeout);
        scrollTimeout = setTimeout(() => {
            loadVisibleElements(visibleElements)
            updateCurrentMediaInURL(visibleElements)
        }, 150); // Wait 150ms after scroll stops
    });

    // Also load images on initial page load
    setTimeout(() => loadVisibleElements(visibleElements), 150);
}

function setHotkeys() {
    const backLink = document.querySelector(".back");

    document.addEventListener("keydown", (e) => {
        switch (e.key) {
            case "ArrowUp":
                backLink.click();
                break;
        }
    });
}
console.log("script")
document.addEventListener("DOMContentLoaded", () => {
    console.log("DOMContentLoaded")
    setHotkeys()
    lazyLoadMedia()
    updateFilter()
    scrollMediaIntoView()
});

// Handle user filter action by setting cookie from filter input
const input = document.getElementById("filter-input");
document.getElementById("set-filter").addEventListener("click", function () {
    setCookie("filter", input.value);
});

document.getElementById("clear-filter").addEventListener("click", function (event) {
    eraseCookie("filter");
    input.value = "";
});
