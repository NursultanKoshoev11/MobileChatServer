(() => {
  const form = document.getElementById('deletion-form');
  const mobileInput = document.getElementById('mobile');
  const codeInput = document.getElementById('code');
  const codeStep = document.getElementById('code-step');
  const sendButton = document.getElementById('send-code');
  const deleteButton = document.getElementById('delete-account');
  const status = document.getElementById('status');
  let verifiedMobile = '';

  function setBusy(busy) {
    sendButton.disabled = busy;
    deleteButton.disabled = busy;
  }

  function showStatus(message) {
    status.textContent = message;
  }

  function normalizeMobile(value) {
    const normalized = value.trim().replace(/[\s()-]/g, '');
    if (/^0\d{9}$/.test(normalized)) return `+996${normalized.slice(1)}`;
    if (/^996\d{9}$/.test(normalized)) return `+${normalized}`;
    return normalized;
  }

  mobileInput.addEventListener('input', () => {
    verifiedMobile = '';
    codeInput.value = '';
    codeStep.hidden = true;
    showStatus('');
  });

  async function readJson(response) {
    try { return await response.json(); } catch (_) { return {}; }
  }

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const mobile = normalizeMobile(mobileInput.value);
    if (!/^\+996\d{9}$/.test(mobile)) {
      showStatus('Введите номер Кыргызстана в формате +996XXXXXXXXX.');
      return;
    }
    setBusy(true);
    showStatus('Отправляем код…');
    try {
      const response = await fetch('/api/auth/request-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mobile }),
      });
      const payload = await readJson(response);
      if (!response.ok) throw new Error(payload.error || 'Не удалось отправить код.');
      verifiedMobile = mobile;
      codeStep.hidden = false;
      showStatus('Если аккаунт с этим номером существует, SMS-код скоро придёт.');
      codeInput.focus();
    } catch (error) {
      showStatus(error.message || 'Не удалось отправить код. Попробуйте позже.');
    } finally {
      setBusy(false);
    }
  });

  deleteButton.addEventListener('click', async () => {
    const code = codeInput.value.trim();
    if (!verifiedMobile || !/^\d{6}$/.test(code)) {
      showStatus('Введите шестизначный код из SMS.');
      return;
    }
    if (!window.confirm('Удалить аккаунт и связанные с ним данные? Публичные обращения и комментарии останутся анонимными.')) return;
    setBusy(true);
    showStatus('Проверяем код…');
    try {
      const verifyResponse = await fetch('/api/auth/verify-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mobile: verifiedMobile, code }),
      });
      const session = await readJson(verifyResponse);
      if (!verifyResponse.ok || !session.access_token) {
        throw new Error('Не удалось подтвердить аккаунт. Проверьте номер и код.');
      }

      showStatus('Удаляем аккаунт…');
      const deleteResponse = await fetch('/api/me', {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${session.access_token}` },
      });
      const result = await readJson(deleteResponse);
      if (deleteResponse.status === 409 && result.error === 'group_ownership_transfer_required') {
        throw new Error('В одной из ваших групп нет другого участника. Войдите в приложение, пригласите человека в группу и повторите удаление.');
      }
      if (!deleteResponse.ok) throw new Error(result.error || 'Не удалось удалить аккаунт.');
      form.hidden = true;
      showStatus('Аккаунт удалён. Публичные обращения и комментарии сохранены анонимно.');
    } catch (error) {
      showStatus(error.message || 'Не удалось удалить аккаунт. Попробуйте позже.');
    } finally {
      setBusy(false);
    }
  });
})();
