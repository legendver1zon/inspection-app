// Кропирование и загрузка плана помещения (CropperJS)
(function() {
    var planFile   = document.getElementById('plan-file');
    var uploadBtn  = document.getElementById('upload-btn');
    var uploadForm = document.getElementById('upload-form');
    var cropModal  = document.getElementById('crop-modal');
    var cropImg    = document.getElementById('crop-img');
    var cropApply  = document.getElementById('crop-apply');
    var cropCancel = document.getElementById('crop-cancel');
    var cropStatus = document.getElementById('crop-status');
    var rotLeft    = document.getElementById('rot-left');
    var rotRight   = document.getElementById('rot-right');
    if (!planFile || !uploadForm) return;

    var cropper = null;
    var croppedBlob = null;
    var origBlobUrl = null;
    var totalRot = 0;

    function initCropper(src) {
        if (cropper) { cropper.destroy(); cropper = null; }
        cropImg.src = src;
        cropModal.style.display = 'flex';
        cropper = new Cropper(cropImg, {
            viewMode: 1,
            autoCropArea: 0.95,
            movable: true,
            zoomable: true,
            rotatable: false,
        });
    }

    planFile.addEventListener('change', function() {
        if (!this.files.length) return;
        if (origBlobUrl) URL.revokeObjectURL(origBlobUrl);
        origBlobUrl = URL.createObjectURL(this.files[0]);
        totalRot = 0;
        initCropper(origBlobUrl);
    });

    function rotateFit(deg) {
        if (!origBlobUrl) return;
        totalRot = (totalRot + deg + 360) % 360;
        var img = new Image();
        img.onload = function() {
            var swap = totalRot === 90 || totalRot === 270;
            var cw = swap ? img.height : img.width;
            var ch = swap ? img.width : img.height;
            var cv = document.createElement('canvas');
            cv.width = cw; cv.height = ch;
            var ctx = cv.getContext('2d');
            ctx.translate(cw / 2, ch / 2);
            ctx.rotate(totalRot * Math.PI / 180);
            ctx.drawImage(img, -img.width / 2, -img.height / 2);
            initCropper(cv.toDataURL('image/jpeg', 0.95));
        };
        img.src = origBlobUrl;
    }
    rotLeft.addEventListener('click',  function() { rotateFit(-90); });
    rotRight.addEventListener('click', function() { rotateFit(90); });

    cropCancel.addEventListener('click', function() {
        cropModal.style.display = 'none';
        if (cropper) { cropper.destroy(); cropper = null; }
        planFile.value = '';
        uploadBtn.disabled = true;
        origBlobUrl = null;
        totalRot = 0;
    });

    cropApply.addEventListener('click', function() {
        if (!cropper) return;
        cropStatus.textContent = 'Обработка...';
        cropApply.disabled = true;
        cropper.getCroppedCanvas({ maxWidth: 2400, maxHeight: 2400 }).toBlob(function(blob) {
            croppedBlob = blob;
            cropModal.style.display = 'none';
            cropper.destroy(); cropper = null;
            cropApply.disabled = false;
            cropStatus.textContent = '';
            uploadBtn.disabled = false;
            var previewWrap = document.getElementById('plan-preview-wrap');
            var preview = document.getElementById('plan-preview');
            if (preview) {
                preview.src = URL.createObjectURL(blob);
                if (previewWrap) previewWrap.style.display = 'block';
            }
        }, 'image/jpeg', 0.92);
    });

    uploadForm.addEventListener('submit', async function(e) {
        e.preventDefault();
        if (!croppedBlob) return;
        uploadBtn.disabled = true;
        uploadBtn.textContent = 'Загрузка...';
        var fd = new FormData();
        fd.append('plan_image', croppedBlob, 'plan.jpg');
        try {
            await fetch(this.action, { method: 'POST', body: fd });
            location.reload();
        } catch(err) {
            uploadBtn.disabled = false;
            uploadBtn.textContent = 'Загрузить фото';
            alert('Ошибка загрузки. Попробуйте ещё раз.');
        }
    });
})();

// Предупреждение при уходе со страницы с несохранёнными изменениями
(function() {
    var form = document.getElementById('main-form');
    if (!form) return;
    var initial = new FormData(form);
    var saved = false;

    function formChanged() {
        var current = new FormData(form);
        for (var [key, val] of current.entries()) {
            if (initial.get(key) !== val) return true;
        }
        for (var [key, val] of initial.entries()) {
            if (current.get(key) !== val) return true;
        }
        return false;
    }

    form.addEventListener('submit', function() { saved = true; });

    window.addEventListener('beforeunload', function(e) {
        if (!saved && formChanged()) {
            e.preventDefault();
            e.returnValue = '';
        }
    });
})();
