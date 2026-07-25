// Автосохранение данных формы в localStorage.
// Защита от потери данных при сбое отправки или обрыве соединения.
(function() {
    var form = document.getElementById('main-form');
    if (!form) return;

    // Ключ: путь страницы (уникален для каждой инспекции)
    var storageKey = 'autosave_' + window.location.pathname;
    var SAVE_INTERVAL = 15000; // 15 секунд

    function collectFormData() {
        var data = {};
        var elements = form.querySelectorAll('input:not([disabled]), select:not([disabled]), textarea:not([disabled])');
        elements.forEach(function(el) {
            if (!el.name) return;
            if (el.type === 'file') return;
            if (el.type === 'checkbox' || el.type === 'radio') {
                if (el.checked) data[el.name] = el.value || 'on';
            } else {
                data[el.name] = el.value;
            }
        });
        return data;
    }

    function saveToStorage() {
        try {
            var data = collectFormData();
            data._savedAt = new Date().toISOString();
            localStorage.setItem(storageKey, JSON.stringify(data));
        } catch (e) {
            // localStorage полон или недоступен — молча игнорируем
        }
    }

    function getSavedData() {
        try {
            var raw = localStorage.getItem(storageKey);
            if (!raw) return null;
            return JSON.parse(raw);
        } catch (e) {
            return null;
        }
    }

    function clearSaved() {
        try { localStorage.removeItem(storageKey); } catch (e) {}
    }

    function restoreFromStorage(data) {
        Object.keys(data).forEach(function(name) {
            if (name === '_savedAt') return;
            var els = form.querySelectorAll('[name="' + CSS.escape(name) + '"]');
            els.forEach(function(el) {
                if (el.type === 'checkbox' || el.type === 'radio') {
                    el.checked = (el.value === data[name] || data[name] === 'on');
                } else if (el.type !== 'file') {
                    el.value = data[name];
                }
            });
        });

        // Обновить счётчик комнат если сохранён
        if (data['active_rooms'] && typeof window.addRoom === 'function') {
            var saved = parseInt(data['active_rooms'], 10);
            var current = parseInt(document.getElementById('active_rooms').value, 10);
            while (current < saved) {
                window.addRoom();
                current++;
            }
        }
    }

    // При загрузке: предложить восстановление если есть черновик
    var saved = getSavedData();
    if (saved && saved._savedAt) {
        var savedDate = new Date(saved._savedAt);
        var age = Date.now() - savedDate.getTime();
        // Показывать только если черновик не старше 24 часов
        if (age < 24 * 60 * 60 * 1000) {
            var time = savedDate.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
            var banner = document.createElement('div');
            banner.className = 'alert alert-warning autosave-banner';
            banner.innerHTML =
                'Найден несохранённый черновик (от ' + time + '). ' +
                '<button type="button" class="btn btn-sm btn-primary" id="restore-draft">Восстановить</button> ' +
                '<button type="button" class="btn btn-sm btn-outline" id="discard-draft">Отклонить</button>';
            form.parentNode.insertBefore(banner, form);

            document.getElementById('restore-draft').addEventListener('click', function() {
                restoreFromStorage(saved);
                banner.remove();
            });
            document.getElementById('discard-draft').addEventListener('click', function() {
                clearSaved();
                banner.remove();
            });
        } else {
            clearSaved();
        }
    }

    // Периодическое автосохранение
    setInterval(saveToStorage, SAVE_INTERVAL);

    // Сохранение при уходе со страницы
    window.addEventListener('beforeunload', saveToStorage);

    // При успешной отправке формы — удалить черновик
    form.addEventListener('submit', function() {
        clearSaved();
    });
})();
