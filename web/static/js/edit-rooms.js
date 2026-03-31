// Управление помещениями и окнами на странице редактирования
(function() {
    var MAX_ROOMS = 10;
    var activeRooms = parseInt(document.getElementById('active_rooms').value, 10) || 1;

    function updateUI() {
        document.getElementById('active_rooms').value = activeRooms;
        var label = document.getElementById('room-count-label');
        label.textContent = activeRooms + ' из ' + MAX_ROOMS + ' помещений';
        document.getElementById('btn-add-room').disabled = activeRooms >= MAX_ROOMS;

        for (var i = 1; i <= MAX_ROOMS; i++) {
            var block = document.getElementById('room-' + i);
            if (!block) continue;
            if (i <= activeRooms) {
                block.classList.remove('room-hidden');
                block.querySelectorAll('input, select, textarea').forEach(function(el) { el.disabled = false; });
            } else {
                block.classList.add('room-hidden');
                block.querySelectorAll('input, select, textarea').forEach(function(el) { el.disabled = true; });
            }
        }
    }

    window.addRoom = function() {
        if (activeRooms >= MAX_ROOMS) return;
        activeRooms++;
        updateUI();
        var newRoom = document.getElementById('room-' + activeRooms);
        if (newRoom) newRoom.scrollIntoView({ behavior: 'smooth', block: 'start' });
    };

    window.removeRoom = function(num) {
        if (activeRooms <= 1) return;
        var block = document.getElementById('room-' + num);
        if (block) {
            block.querySelectorAll('input[type=text], input[type=number]').forEach(function(el) { el.value = ''; });
            block.querySelectorAll('input[type=radio], input[type=checkbox]').forEach(function(el) { el.checked = false; });
        }
        for (var i = num; i < activeRooms; i++) {
            swapRoomData(i, i + 1);
        }
        activeRooms--;
        updateUI();
    };

    function swapRoomData(to, from) {
        var toBlock = document.getElementById('room-' + to);
        var fromBlock = document.getElementById('room-' + from);
        if (!toBlock || !fromBlock) return;

        var toInputs = toBlock.querySelectorAll('input, select, textarea');
        var fromInputs = fromBlock.querySelectorAll('input, select, textarea');

        fromInputs.forEach(function(fromEl, idx) {
            var toEl = toInputs[idx];
            if (!toEl) return;
            if (fromEl.type === 'radio' || fromEl.type === 'checkbox') {
                toEl.checked = fromEl.checked;
            } else {
                toEl.value = fromEl.value;
            }
        });
    }

    // ===== Динамические окна =====
    function initWindows() {
        for (var r = 1; r <= MAX_ROOMS; r++) {
            var maxWin = 1;
            for (var w = 2; w <= 5; w++) {
                var hEl = document.querySelector('input[name="room_w' + w + 'h_' + r + '"]');
                var wEl = document.querySelector('input[name="room_w' + w + 'w_' + r + '"]');
                if ((hEl && hEl.value) || (wEl && wEl.value)) {
                    maxWin = w;
                }
            }
            for (var w = 2; w <= maxWin; w++) {
                showWindowPair(r, w);
            }
            updateWinBtn(r);
        }
    }

    function showWindowPair(roomNum, winNum) {
        document.querySelectorAll('.extra-win[data-room="' + roomNum + '"][data-wnum="' + winNum + '"]')
            .forEach(function(el) { el.style.display = ''; });
    }

    window.addWindow = function(roomNum) {
        for (var w = 2; w <= 5; w++) {
            var pairs = document.querySelectorAll('.extra-win[data-room="' + roomNum + '"][data-wnum="' + w + '"]');
            if (pairs.length > 0 && pairs[0].style.display === 'none') {
                pairs.forEach(function(el) { el.style.display = ''; });
                updateWinBtn(roomNum);
                return;
            }
        }
    };

    function updateWinBtn(roomNum) {
        var btn = document.getElementById('btn-win-' + roomNum);
        if (!btn) return;
        var w5 = document.querySelectorAll('.extra-win[data-room="' + roomNum + '"][data-wnum="5"]');
        var maxReached = w5.length > 0 && w5[0].style.display !== 'none';
        btn.disabled = maxReached;
        btn.style.opacity = maxReached ? '0.4' : '';
    }

    // ===== Accordion: сворачивание/разворачивание комнат =====
    window.toggleRoom = function(roomNum, event) {
        var body = document.getElementById('room-body-' + roomNum);
        var icon = document.getElementById('room-icon-' + roomNum);
        if (!body) return;
        if (body.style.display === 'none') {
            body.style.display = '';
            if (icon) icon.textContent = '▼';
        } else {
            body.style.display = 'none';
            if (icon) icon.textContent = '►';
        }
    };

    updateUI();
    initWindows();
})();
