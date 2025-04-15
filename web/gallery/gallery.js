const backLink = document.querySelector(".back");

document.addEventListener("keydown", (e) => {
    switch (e.key) {
        case "ArrowUp":
            backLink.click();
            break;
    }
});

document.addEventListener("DOMContentLoaded", () => {
    const lazyMediaEls = document.querySelectorAll(".lazy");
    let scrollTimeout;
    let visibleElements = new Set();

    // Function to load visible elements
    function loadVisibleElements() {
        visibleElements.forEach(el => {
            el.setAttribute("src", el.dataset.url);
            visibleElements.delete(el);
        });
    }

    if ("IntersectionObserver" in window) {
        let mediaLoadObs = new IntersectionObserver(function (entries, observer) {

            entries.forEach((entry) => {
                if (entry.isIntersecting) {
                    visibleElements.add(entry.target);
                } else {
                    visibleElements.delete(entry.target);
                }
            });
        }, {
            rootMargin: '100px 0px', // Increase buffer zone since we're loading on scroll stop
            threshold: 0.1
        });

        lazyMediaEls.forEach((el) => {
            mediaLoadObs.observe(el);
        });

        // Load images when scrolling stops
        window.addEventListener('scroll', () => {
            clearTimeout(scrollTimeout);
            scrollTimeout = setTimeout(loadVisibleElements, 150); // Wait 150ms after scroll stops
        });

        // Also load images on initial page load
        setTimeout(loadVisibleElements, 150);
    }


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

    // Restore scroll position from URL parameter
    const curMedia = searchParams.get('p')

    if (curMedia) {
        const allMedia = document.querySelectorAll(".lazy");
        allMedia.forEach((m) => {
            const mediaName = m.dataset.url.split("/").slice(-1).pop()
            if (mediaName === curMedia) {
                m.scrollIntoView()
            }
        })
    }
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

