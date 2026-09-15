const STORAGE_KEY = "gallery-settings";

const defaults = {
    gridSize: 300,
    sortOrder: "newest",
    dirsFirst: true,
};

function loadSettings() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (raw) {
            return { ...defaults, ...JSON.parse(raw) };
        }
    } catch (e) {}
    return { ...defaults };
}

function saveSettings(settings) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}

function populateForm(settings) {
    const gridInput = document.getElementById("grid-size");
    const gridLabel = document.getElementById("grid-size-label");
    const sortSelect = document.getElementById("sort-order");
    const dirsCheck = document.getElementById("dirs-first");

    gridInput.value = settings.gridSize;
    gridLabel.textContent = settings.gridSize + "px";
    sortSelect.value = settings.sortOrder;
    dirsCheck.checked = settings.dirsFirst;
}

function readForm() {
    return {
        gridSize: parseInt(document.getElementById("grid-size").value, 10),
        sortOrder: document.getElementById("sort-order").value,
        dirsFirst: document.getElementById("dirs-first").checked,
    };
}

document.addEventListener("DOMContentLoaded", () => {
    const settings = loadSettings();
    populateForm(settings);

    const gridInput = document.getElementById("grid-size");
    const gridLabel = document.getElementById("grid-size-label");

    gridInput.addEventListener("input", () => {
        gridLabel.textContent = gridInput.value + "px";
    });

    document.getElementById("settings-form").addEventListener("submit", (e) => {
        e.preventDefault();
        const newSettings = readForm();
        saveSettings(newSettings);
        const msg = document.getElementById("save-msg");
        msg.textContent = "Saved";
        setTimeout(() => { msg.textContent = ""; }, 1500);
    });

    document.getElementById("reset-btn").addEventListener("click", () => {
        saveSettings({ ...defaults });
        populateForm(defaults);
    });
});
