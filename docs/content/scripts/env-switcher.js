(function () {
  const STORAGE_KEY = "piri-docs-env";
  const DEFAULT_ENV = "production";
  const ENVIRONMENTS = ["production", "staging"];

  function createPicker(initialEnv) {
    const header = document.querySelector(".md-header__inner");
    if (!header) return null;

    const wrapper = document.createElement("div");
    wrapper.className = "env-picker";

    const icon = document.createElement("img");
    icon.className = "env-picker__icon";
    icon.src = "/piri/images/icons/globe.svg";

    const label = document.createElement("span");
    label.className = "env-picker__label";
    label.textContent = "Environment";

    const select = document.createElement("select");
    select.setAttribute("aria-label", "Environment selector");
    select.innerHTML = `
      <option value="production">Production</option>
      <option value="staging">Staging</option>
    `;

    select.value = initialEnv || "";


    wrapper.append(icon, label, select);
    header.append(wrapper);

    return select;
  }

  function applyEnv(env) {
    const safeEnv = ENVIRONMENTS.includes(env) ? env : DEFAULT_ENV;
    document.body.classList.remove(
      "env-production",
      "env-staging"
    );
    document.body.classList.add(`env-${safeEnv}`);

    document.querySelectorAll("[data-env]").forEach((node) => {
      const envs = (node.dataset.env || "")
        .split(",")
        .map((v) => v.trim().toLowerCase())
        .filter(Boolean);
      const matches = envs.length === 0 || envs.includes(safeEnv);
      node.style.display = matches ? "" : "none";
    });
  }

  function init() {
    const saved = localStorage.getItem(STORAGE_KEY) || "";
    const initialEnv = ENVIRONMENTS.includes(saved) ? saved : DEFAULT_ENV;
    const picker = createPicker(initialEnv);

    applyEnv(initialEnv);

    if (picker) {
      picker.addEventListener("change", () => {
        const value = picker.value;
        if (ENVIRONMENTS.includes(value)) {
          localStorage.setItem(STORAGE_KEY, value);
          applyEnv(value);
        } else {
          localStorage.removeItem(STORAGE_KEY);
          applyEnv(DEFAULT_ENV);
        }
      });
    }
  }

  // Apply env class early to avoid flicker/layout jumps
  const earlySaved = (typeof localStorage !== "undefined" && localStorage.getItem(STORAGE_KEY)) || "";
  const earlyEnv = ENVIRONMENTS.includes(earlySaved) ? earlySaved : DEFAULT_ENV;
  document.documentElement.classList.add(`env-${earlyEnv}`);
  document.body.classList.add(`env-${earlyEnv}`);

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
