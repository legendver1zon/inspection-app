// Проверка уникальности номера акта через AJAX.
// При потере фокуса поля act_number → fetch на /api/inspections/:id/check-act-number.
// Если номер занят — показываем предупреждение и блокируем кнопку "Сохранить".
(function () {
    "use strict";

    var input = document.getElementById("act_number_input");
    if (!input) return;

    var warning = document.getElementById("act_number_warning");
    var form = document.getElementById("main-form");
    var submitBtn = form ? form.querySelector('button[type="submit"]') : null;
    var original = input.dataset.original || "";
    var inspectionID = input.dataset.inspectionId;
    if (!inspectionID) return;

    var lastChecked = null;
    var pending = null;

    function showWarning(text) {
        warning.textContent = text;
        warning.style.display = "block";
        input.classList.add("input-invalid");
        if (submitBtn) submitBtn.disabled = true;
    }

    function clearWarning() {
        warning.textContent = "";
        warning.style.display = "none";
        input.classList.remove("input-invalid");
        if (submitBtn) submitBtn.disabled = false;
    }

    function check() {
        var value = input.value.trim();

        if (value === "") {
            showWarning("Номер акта не может быть пустым");
            return;
        }
        if (value === original) {
            clearWarning();
            return;
        }
        if (value === lastChecked) return;

        if (pending) pending.abort();
        var controller = typeof AbortController !== "undefined" ? new AbortController() : null;
        pending = controller;

        var url = "/api/inspections/" + encodeURIComponent(inspectionID) +
            "/check-act-number?value=" + encodeURIComponent(value);

        fetch(url, {
            credentials: "same-origin",
            signal: controller ? controller.signal : undefined,
        })
            .then(function (resp) {
                if (!resp.ok) throw new Error("HTTP " + resp.status);
                return resp.json();
            })
            .then(function (data) {
                lastChecked = value;
                if (data.taken) {
                    showWarning('Номер "' + value + '" уже используется в осмотре #' + data.other_id);
                } else {
                    clearWarning();
                }
            })
            .catch(function (err) {
                if (err.name === "AbortError") return;
                // При сетевой ошибке не блокируем сохранение — бэк всё равно провалидирует.
                clearWarning();
            });
    }

    input.addEventListener("blur", check);
    input.addEventListener("input", function () {
        // Пока пользователь печатает — убираем предупреждение (проверим снова на blur).
        if (input.value.trim() === original) clearWarning();
    });
})();
