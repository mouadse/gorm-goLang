const state = {
  meta: null,
  selectedFocus: new Set(),
};

const healthPill = document.getElementById("health-pill");
const rebuildButton = document.getElementById("rebuild-button");
const heroGallery = document.getElementById("hero-gallery");
const librarySize = document.getElementById("library-size");
const libraryMuscles = document.getElementById("library-muscles");
const libraryCategories = document.getElementById("library-categories");
const focusGrid = document.getElementById("focus-grid");
const programForm = document.getElementById("program-form");
const programButton = document.getElementById("program-button");
const programStatus = document.getElementById("program-status");
const programSummary = document.getElementById("program-summary");
const recoveryNote = document.getElementById("recovery-note");
const warmupList = document.getElementById("warmup-list");
const programDays = document.getElementById("program-days");
const equipmentProfileSelect = document.getElementById("equipment-profile");
const searchForm = document.getElementById("search-form");
const searchButton = document.getElementById("search-button");
const resultsCount = document.getElementById("results-count");
const resultsGrid = document.getElementById("results-grid");
const atlasStatus = document.getElementById("atlas-status");
const sampleQueryRow = document.getElementById("sample-query-row");
const searchLevel = document.getElementById("search-level");
const searchEquipment = document.getElementById("search-equipment");
const searchCategory = document.getElementById("search-category");
const searchMuscle = document.getElementById("search-muscle");

function escapeHtml(value) {
  const div = document.createElement("div");
  div.textContent = value ?? "";
  return div.innerHTML;
}

function capitalizeWords(value) {
  return String(value || "")
    .split(/[\s-]+/)
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join(" ");
}

function setHealth(label, tone = "idle") {
  healthPill.textContent = label;
  healthPill.dataset.tone = tone;
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function populateSelect(select, values, labels = {}) {
  const current = select.value;
  select.innerHTML = '<option value="">All</option>';
  values.forEach((value) => {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = labels[value] || capitalizeWords(value);
    select.append(option);
  });
  if (values.includes(current)) {
    select.value = current;
  }
}

function renderSpotlights(spotlights) {
  if (!spotlights.length) {
    heroGallery.innerHTML = `
      <article class="gallery-card gallery-card--wide">
        <div class="gallery-copy">
          <p>No exercise spotlights available.</p>
        </div>
      </article>
    `;
    return;
  }

  heroGallery.innerHTML = spotlights
    .map(
      (item, index) => `
        <article class="gallery-card ${index === 0 ? "gallery-card--wide" : ""}">
          <div class="gallery-media">
            ${item.image_url ? `<img src="${escapeHtml(item.image_url)}" alt="${escapeHtml(item.name)}" loading="lazy" />` : ""}
          </div>
          <div class="gallery-copy">
            <span>${escapeHtml(item.category)}</span>
            <h4>${escapeHtml(item.name)}</h4>
            <p>${escapeHtml(item.match_reasons.join(" · "))}</p>
          </div>
        </article>
      `,
    )
    .join("");
}

function renderFocusButtons(muscles) {
  const focusOptions = muscles.slice(0, 10);
  focusGrid.innerHTML = focusOptions
    .map(
      (muscle) => `
        <button class="focus-chip" type="button" data-muscle="${escapeHtml(muscle)}">
          ${escapeHtml(capitalizeWords(muscle))}
        </button>
      `,
    )
    .join("");

  focusGrid.querySelectorAll(".focus-chip").forEach((button) => {
    button.addEventListener("click", () => {
      const muscle = button.dataset.muscle;
      if (!muscle) return;

      if (state.selectedFocus.has(muscle)) {
        state.selectedFocus.delete(muscle);
      } else if (state.selectedFocus.size < 3) {
        state.selectedFocus.add(muscle);
      }

      focusGrid.querySelectorAll(".focus-chip").forEach((chip) => {
        chip.classList.toggle("is-active", state.selectedFocus.has(chip.dataset.muscle));
      });
    });
  });
}

function renderEquipmentProfiles(profiles) {
  equipmentProfileSelect.innerHTML = profiles
    .map(
      (profile) =>
        `<option value="${escapeHtml(profile.value)}"${profile.value === "home-bodyweight" ? " selected" : ""}>${escapeHtml(profile.label)}</option>`,
    )
    .join("");
}

function renderSampleQueries(queries) {
  sampleQueryRow.innerHTML = queries
    .map((query) => `<button class="sample-chip" type="button" data-query="${escapeHtml(query)}">${escapeHtml(query)}</button>`)
    .join("");

  sampleQueryRow.querySelectorAll(".sample-chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      const input = document.getElementById("query");
      input.value = chip.dataset.query || "";
      runSearch(input.value);
    });
  });
}

function renderWarmup(items) {
  warmupList.innerHTML = items.map((item) => `<li>${escapeHtml(item)}</li>`).join("");
}

function renderProgramDays(days) {
  if (!days.length) {
    programDays.innerHTML = `
      <article class="empty-state">
        <p class="empty-state__eyebrow">No days returned</p>
        <h4>The planner did not return a split.</h4>
        <p>Adjust the setup and try again.</p>
      </article>
    `;
    return;
  }

  programDays.innerHTML = days
    .map(
      (day) => `
        <article class="day-card">
          <div class="day-header">
            <div>
              <span>Day ${day.day}</span>
              <h4>${escapeHtml(day.title)}</h4>
            </div>
            <p>${escapeHtml(day.duration_label)}</p>
          </div>
          <p class="day-focus">${escapeHtml(day.focus)}</p>
          <div class="day-exercise-list">
            ${day.exercises
              .map(
                (exercise) => `
                  <article class="exercise-row">
                    <div class="exercise-row__media">
                      ${exercise.image_url ? `<img src="${escapeHtml(exercise.image_url)}" alt="${escapeHtml(exercise.name)}" loading="lazy" />` : ""}
                      ${
                        exercise.alt_image_url
                          ? `<img class="exercise-row__media-alt" src="${escapeHtml(exercise.alt_image_url)}" alt="" loading="lazy" />`
                          : ""
                      }
                    </div>
                    <div class="exercise-row__copy">
                      <div class="exercise-row__topline">
                        <h5>${escapeHtml(exercise.name)}</h5>
                        <span>${escapeHtml(exercise.prescription)}</span>
                      </div>
                      <p>${escapeHtml(exercise.reason)}</p>
                      <div class="chip-cluster">
                        <span>${escapeHtml(exercise.category)}</span>
                        <span>${escapeHtml(exercise.equipment)}</span>
                        ${exercise.primary_muscles
                          .slice(0, 2)
                          .map((muscle) => `<span>${escapeHtml(muscle)}</span>`)
                          .join("")}
                      </div>
                      <details>
                        <summary>Technique notes</summary>
                        <ol>
                          ${exercise.instructions.slice(0, 4).map((step) => `<li>${escapeHtml(step)}</li>`).join("")}
                        </ol>
                      </details>
                    </div>
                  </article>
                `,
              )
              .join("")}
          </div>
        </article>
      `,
    )
    .join("");
}

function renderResults(results, query) {
  if (!results.length) {
    resultsCount.textContent = "0 matches";
    resultsGrid.innerHTML = `
      <article class="empty-state">
        <p class="empty-state__eyebrow">No direct matches</p>
        <h4>Nothing aligned with "${escapeHtml(query)}".</h4>
        <p>Broaden the query or loosen one of the filters.</p>
      </article>
    `;
    return;
  }

  const strongestScore = Math.max(...results.map((item) => Math.max(item.score, 0)), 0);
  resultsCount.textContent = `${results.length} matches`;
  resultsGrid.innerHTML = results
    .map(
      (item, index) => {
        const relativeScore = strongestScore > 0 ? Math.round((Math.max(item.score, 0) / strongestScore) * 100) : 0;
        return `
        <article class="result-card" style="--delay:${index * 0.05}s">
          <div class="result-card__media">
            ${item.image_url ? `<img src="${escapeHtml(item.image_url)}" alt="${escapeHtml(item.name)}" loading="lazy" />` : ""}
            ${
              item.alt_image_url
                ? `<img class="result-card__media-alt" src="${escapeHtml(item.alt_image_url)}" alt="" loading="lazy" />`
                : ""
            }
          </div>
          <div class="result-card__copy">
            <div class="result-card__header">
              <div>
                <p>${escapeHtml(item.category)} · ${escapeHtml(item.level)}</p>
                <h4>${escapeHtml(item.name)}</h4>
              </div>
              <strong>${relativeScore}%</strong>
            </div>
            <p class="result-card__meta">${escapeHtml(item.equipment)} · ${escapeHtml(item.force)} · ${escapeHtml(item.mechanic)}</p>
            <div class="chip-cluster">
              ${item.match_reasons.map((reason) => `<span>${escapeHtml(reason)}</span>`).join("")}
            </div>
            <p class="result-card__instructions">${escapeHtml((item.instructions[0] || "No instruction preview available.").trim())}</p>
            <details>
              <summary>See steps</summary>
              <ol>
                ${item.instructions.slice(0, 4).map((step) => `<li>${escapeHtml(step)}</li>`).join("")}
              </ol>
            </details>
          </div>
        </article>
      `;
      },
    )
    .join("");
}

async function refreshHealth() {
  setHealth("Checking...", "idle");
  try {
    const response = await fetch("/health");
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const payload = await response.json();
    if (payload.catalog_status === "ready") {
      setHealth(`Atlas ready · ${payload.exercises_loaded}`, "ok");
    } else if (payload.catalog_status === "error") {
      setHealth("Catalog failed", "error");
    } else {
      setHealth("Booting catalog", "idle");
    }
    return payload;
  } catch (error) {
    setHealth("Backend offline", "error");
    return null;
  }
}

async function waitForCatalogReady() {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const payload = await refreshHealth();
    if (payload?.ready) {
      return payload;
    }
    if (payload?.catalog_status === "error") {
      throw new Error(payload.error || "Catalog initialization failed.");
    }
    await sleep(1000);
  }
  throw new Error("Catalog initialization is taking longer than expected.");
}

async function loadMeta() {
  const response = await fetch("/catalog/meta");
  if (!response.ok) {
    throw new Error(`Failed to load catalog metadata (${response.status})`);
  }

  const meta = await response.json();
  state.meta = meta;
  librarySize.textContent = String(meta.library_size);
  libraryMuscles.textContent = String(meta.muscles.length);
  libraryCategories.textContent = String(meta.categories.length);

  renderSpotlights(meta.spotlights);
  renderFocusButtons(meta.muscles);
  renderEquipmentProfiles(meta.equipment_profiles);
  renderSampleQueries(meta.sample_queries);
  populateSelect(searchLevel, meta.levels);
  populateSelect(searchEquipment, meta.equipment);
  populateSelect(searchCategory, meta.categories);
  populateSelect(searchMuscle, meta.muscles);
}

async function rebuildCatalog() {
  rebuildButton.disabled = true;
  rebuildButton.textContent = "Rebuilding...";
  setHealth("Reindexing...", "idle");

  try {
    const response = await fetch("/init", { method: "POST" });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.detail || `HTTP ${response.status}`);
    await loadMeta();
    await refreshHealth();
    atlasStatus.textContent = `Rebuilt local atlas with ${payload.exercises_loaded} exercises.`;
    programStatus.textContent = "Atlas rebuilt. You can generate a fresh split.";
  } catch (error) {
    setHealth("Rebuild failed", "error");
    atlasStatus.textContent = String(error);
  } finally {
    rebuildButton.disabled = false;
    rebuildButton.textContent = "Rebuild Atlas";
  }
}

async function buildProgram() {
  const formData = new FormData(programForm);
  const payload = {
    goal: formData.get("goal"),
    days_per_week: Number(formData.get("days_per_week")),
    session_minutes: Number(formData.get("session_minutes")),
    level: formData.get("level"),
    equipment_profile: formData.get("equipment_profile"),
    notes: String(formData.get("notes") || "").trim(),
    focus: Array.from(state.selectedFocus),
  };

  programButton.disabled = true;
  programButton.textContent = "Building...";
  programStatus.textContent = "Scoring local matches and composing the week.";
  programDays.innerHTML = `
    <article class="empty-state">
      <p class="empty-state__eyebrow">Program in progress</p>
      <h4>Finding the best split for this setup.</h4>
      <p>The backend is ranking exercises from the local image-backed catalog.</p>
    </article>
  `;

  try {
    const response = await fetch("/program", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.detail || `HTTP ${response.status}`);

    programSummary.textContent = result.summary;
    recoveryNote.textContent = result.recovery_note;
    renderWarmup(result.warmup);
    renderProgramDays(result.days);
    programStatus.textContent = "Weekly split generated from the local exercise atlas.";
  } catch (error) {
    programSummary.textContent = "Program generation failed.";
    recoveryNote.textContent = String(error);
    renderWarmup(["Retry after adjusting the setup or rebuilding the local catalog."]);
    renderProgramDays([]);
    programStatus.textContent = "Planner request failed.";
  } finally {
    programButton.disabled = false;
    programButton.textContent = "Build My Split";
  }
}

async function runSearch(queryOverride = "") {
  const queryInput = document.getElementById("query");
  const query = (queryOverride || queryInput.value || "").trim();
  if (!query) {
    queryInput.focus();
    return;
  }

  const payload = {
    query,
    top_k: Number(document.getElementById("top-k").value || 9),
    level: searchLevel.value || null,
    equipment: searchEquipment.value || null,
    category: searchCategory.value || null,
    muscle: searchMuscle.value || null,
  };

  searchButton.disabled = true;
  searchButton.textContent = "Searching...";
  atlasStatus.textContent = "Querying the local vector index.";
  resultsGrid.innerHTML = `
    <article class="empty-state">
      <p class="empty-state__eyebrow">Search running</p>
      <h4>Ranking exercises against your prompt.</h4>
      <p>This is using the local catalog, not a remote media API.</p>
    </article>
  `;

  try {
    const response = await fetch("/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.detail || `HTTP ${response.status}`);

    atlasStatus.textContent = `Search complete for "${query}".`;
    renderResults(result.results, query);
  } catch (error) {
    atlasStatus.textContent = String(error);
    renderResults([], query);
  } finally {
    searchButton.disabled = false;
    searchButton.textContent = "Search Atlas";
  }
}

programForm.addEventListener("submit", (event) => {
  event.preventDefault();
  buildProgram();
});

searchForm.addEventListener("submit", (event) => {
  event.preventDefault();
  runSearch();
});

rebuildButton.addEventListener("click", rebuildCatalog);

async function init() {
  try {
    await waitForCatalogReady();
    await loadMeta();
  } catch (error) {
    setHealth("Catalog unavailable", "error");
    programStatus.textContent = String(error);
    atlasStatus.textContent = String(error);
  }
}

init();
