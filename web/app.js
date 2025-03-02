/**
 * AetherWave Web Interface
 * JavaScript для взаимодействия с API узлов блокчейн-сети
 */

// Состояние приложения
const state = {
    connected: false,
    nodeAddress: '',
    encryptionKey: '',
    username: '',
};

// DOM элементы
const elements = {
    connectBtn: document.getElementById('connect-btn'),
    nodeAddress: document.getElementById('node-address'),
    encryptionKey: document.getElementById('encryption-key'),
    username: document.getElementById('username'),
    sendForm: document.getElementById('send-form'),
    recipient: document.getElementById('recipient'),
    message: document.getElementById('message'),
    messages: document.getElementById('messages'),
    refreshMessages: document.getElementById('refresh-messages'),
    refreshBlockchain: document.getElementById('refresh-blockchain'),
    refreshPeers: document.getElementById('refresh-peers'),
    blockchainInfo: document.getElementById('blockchain-info'),
    peersList: document.getElementById('peers-list'),
};

// Установка обработчиков событий
document.addEventListener('DOMContentLoaded', () => {
    // Инициализация обработчиков
    elements.connectBtn.addEventListener('click', handleConnect);
    elements.sendForm.addEventListener('submit', handleSendMessage);
    elements.refreshMessages.addEventListener('click', fetchMessages);
    elements.refreshBlockchain.addEventListener('click', fetchBlockchainInfo);
    elements.refreshPeers.addEventListener('click', fetchPeers);
    
    // Проверка localStorage для автоматического подключения
    const savedNodeAddress = localStorage.getItem('nodeAddress');
    const savedEncryptionKey = localStorage.getItem('encryptionKey');
    const savedUsername = localStorage.getItem('username');
    
    if (savedNodeAddress && savedEncryptionKey && savedUsername) {
        elements.nodeAddress.value = savedNodeAddress;
        elements.encryptionKey.value = savedEncryptionKey;
        elements.username.value = savedUsername;
        handleConnect();
    }
});

// Обработчик подключения к узлу
async function handleConnect() {
    const nodeAddress = elements.nodeAddress.value.trim();
    const encryptionKey = elements.encryptionKey.value.trim();
    const username = elements.username.value.trim();
    
    if (!nodeAddress) {
        showAlert('Ошибка', 'Укажите адрес узла', 'danger');
        return;
    }
    
    if (!username) {
        showAlert('Ошибка', 'Укажите имя пользователя', 'danger');
        return;
    }
    
    // Если ключ не указан, генерируем новый
    let key = encryptionKey;
    if (!encryptionKey) {
        try {
            key = await generateEncryptionKey();
            elements.encryptionKey.value = key;
            showAlert('Информация', `Сгенерирован новый ключ шифрования: ${key}`, 'info');
        } catch (error) {
            showAlert('Ошибка', `Не удалось сгенерировать ключ: ${error.message}`, 'danger');
            return;
        }
    }
    
    // Пробуем подключиться к узлу
    try {
        elements.connectBtn.disabled = true;
        elements.connectBtn.textContent = 'Подключение...';
        
        // Сохраняем состояние приложения
        state.nodeAddress = nodeAddress;
        state.encryptionKey = key;
        state.username = username;
        
        // Сохраняем в localStorage
        localStorage.setItem('nodeAddress', nodeAddress);
        localStorage.setItem('encryptionKey', key);
        localStorage.setItem('username', username);
        
        // Получаем информацию о блокчейне для проверки подключения
        await fetchBlockchainInfo();
        
        // Если дошли до этой точки, значит, подключение успешно
        state.connected = true;
        elements.connectBtn.textContent = 'Подключено';
        elements.connectBtn.classList.remove('btn-primary');
        elements.connectBtn.classList.add('btn-success');
        
        // Загружаем данные
        await Promise.all([
            fetchMessages(),
            fetchPeers()
        ]);
        
        showAlert('Успех', `Подключено к узлу ${nodeAddress}`, 'success');
    } catch (error) {
        showAlert('Ошибка подключения', error.message, 'danger');
        elements.connectBtn.textContent = 'Подключиться';
        elements.connectBtn.disabled = false;
    }
}

// Обработчик отправки сообщения
async function handleSendMessage(event) {
    event.preventDefault();
    
    if (!state.connected) {
        showAlert('Ошибка', 'Сначала подключитесь к узлу', 'warning');
        return;
    }
    
    const recipient = elements.recipient.value.trim();
    const messageText = elements.message.value.trim();
    
    if (!recipient || !messageText) {
        showAlert('Ошибка', 'Заполните все поля формы', 'warning');
        return;
    }
    
    try {
        const submitBtn = elements.sendForm.querySelector('button[type="submit"]');
        submitBtn.disabled = true;
        submitBtn.textContent = 'Отправка...';
        
        const response = await fetch(`http://${state.nodeAddress}/api/message`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                sender: state.username,
                recipient: recipient,
                content: messageText,
                key: state.encryptionKey
            })
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        
        // Очищаем форму
        elements.message.value = '';
        
        // Обновляем данные
        await Promise.all([
            fetchMessages(),
            fetchBlockchainInfo()
        ]);
        
        showAlert('Успех', 'Сообщение отправлено и добавлено в блокчейн', 'success');
    } catch (error) {
        showAlert('Ошибка отправки', error.message, 'danger');
    } finally {
        const submitBtn = elements.sendForm.querySelector('button[type="submit"]');
        submitBtn.disabled = false;
        submitBtn.textContent = 'Отправить';
    }
}

// Функция для получения сообщений
async function fetchMessages() {
    if (!state.connected) {
        showAlert('Ошибка', 'Сначала подключитесь к узлу', 'warning');
        return;
    }
    
    try {
        elements.refreshMessages.disabled = true;
        elements.messages.innerHTML = '<div class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">Загрузка...</span></div></div>';
        
        const response = await fetch(`http://${state.nodeAddress}/api/messages?username=${state.username}&key=${state.encryptionKey}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const messages = await response.json();
        
        // Обновляем UI
        if (messages.length === 0) {
            elements.messages.innerHTML = '<div class="alert alert-info">У вас нет сообщений</div>';
        } else {
            elements.messages.innerHTML = '';
            messages.forEach(msg => {
                const isOutgoing = msg.sender === state.username;
                const messageEl = document.createElement('div');
                messageEl.className = isOutgoing ? 'message-item outgoing' : 'message-item';
                messageEl.innerHTML = `
                    <div class="d-flex justify-content-between">
                        <strong>${isOutgoing ? 'Вы → ' + msg.recipient : msg.sender}</strong>
                        <small class="text-muted">${formatTimestamp(msg.timestamp)}</small>
                    </div>
                    <p class="mb-0">${msg.content}</p>
                `;
                elements.messages.appendChild(messageEl);
            });
        }
    } catch (error) {
        showAlert('Ошибка получения сообщений', error.message, 'danger');
        elements.messages.innerHTML = '<div class="alert alert-danger">Не удалось загрузить сообщения</div>';
    } finally {
        elements.refreshMessages.disabled = false;
    }
}

// Функция для получения информации о блокчейне
async function fetchBlockchainInfo() {
    try {
        elements.refreshBlockchain.disabled = true;
        elements.blockchainInfo.innerHTML = '<div class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">Загрузка...</span></div></div>';
        
        const response = await fetch(`http://${state.nodeAddress}/api/blockchain`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const data = await response.json();
        
        // Обновляем UI
        elements.blockchainInfo.innerHTML = `
            <div class="row text-start">
                <div class="col-md-4">
                    <h5>Общая информация</h5>
                    <p><strong>Количество блоков:</strong> ${data.blockCount}</p>
                    <p><strong>Сложность:</strong> ${data.difficulty}</p>
                    <p><strong>Всего сообщений:</strong> ${data.messageCount}</p>
                </div>
                <div class="col-md-8">
                    <h5>Последний блок</h5>
                    <p><strong>Индекс:</strong> ${data.lastBlock.index}</p>
                    <p><strong>Хеш:</strong> <code>${data.lastBlock.hash}</code></p>
                    <p><strong>Предыдущий хеш:</strong> <code>${data.lastBlock.prevHash}</code></p>
                    <p><strong>Nonce:</strong> ${data.lastBlock.nonce}</p>
                    <p><strong>Timestamp:</strong> ${formatTimestamp(data.lastBlock.timestamp)}</p>
                    <p><strong>Сообщений в блоке:</strong> ${data.lastBlock.messageCount}</p>
                </div>
            </div>
        `;
    } catch (error) {
        showAlert('Ошибка получения данных блокчейна', error.message, 'danger');
        elements.blockchainInfo.innerHTML = '<div class="alert alert-danger">Не удалось загрузить информацию о блокчейне</div>';
    } finally {
        elements.refreshBlockchain.disabled = false;
    }
}

// Функция для получения списка пиров
async function fetchPeers() {
    if (!state.connected) {
        showAlert('Ошибка', 'Сначала подключитесь к узлу', 'warning');
        return;
    }
    
    try {
        elements.refreshPeers.disabled = true;
        elements.peersList.innerHTML = '<div class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">Загрузка...</span></div></div>';
        
        const response = await fetch(`http://${state.nodeAddress}/api/peers`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const peers = await response.json();
        
        // Обновляем UI
        if (peers.length === 0) {
            elements.peersList.innerHTML = '<div class="alert alert-info">Нет активных пиров</div>';
        } else {
            elements.peersList.innerHTML = `
                <div class="table-responsive">
                    <table class="table table-striped">
                        <thead>
                            <tr>
                                <th>Адрес</th>
                                <th>Статус</th>
                                <th>Последняя активность</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${peers.map(peer => `
                                <tr>
                                    <td>${peer.address}</td>
                                    <td>
                                        <span class="badge ${peer.active ? 'bg-success' : 'bg-danger'}">
                                            ${peer.active ? 'Активен' : 'Неактивен'}
                                        </span>
                                    </td>
                                    <td>${formatTimestamp(peer.lastSeen)}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            `;
        }
    } catch (error) {
        showAlert('Ошибка получения списка пиров', error.message, 'danger');
        elements.peersList.innerHTML = '<div class="alert alert-danger">Не удалось загрузить список пиров</div>';
    } finally {
        elements.refreshPeers.disabled = false;
    }
}

// Вспомогательная функция для форматирования временной метки
function formatTimestamp(timestamp) {
    const date = new Date(timestamp * 1000);
    return date.toLocaleString();
}

// Функция для генерации ключа шифрования
async function generateEncryptionKey() {
    // Генерируем 32 байта случайных данных
    const array = new Uint8Array(32);
    window.crypto.getRandomValues(array);
    
    // Преобразуем в hex-строку
    return Array.from(array)
        .map(b => b.toString(16).padStart(2, '0'))
        .join('');
}

// Функция для отображения оповещений
function showAlert(title, message, type = 'info') {
    const alertId = 'app-alert-' + Date.now();
    const alertHtml = `
        <div id="${alertId}" class="alert alert-${type} alert-dismissible fade show" role="alert">
            <strong>${title}</strong> ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
        </div>
    `;
    
    // Добавляем оповещение в верхнюю часть страницы
    const alertContainer = document.createElement('div');
    alertContainer.className = 'container mt-3';
    alertContainer.innerHTML = alertHtml;
    document.body.insertBefore(alertContainer, document.body.firstChild);
    
    // Автоматически скрываем через 5 секунд
    setTimeout(() => {
        const alertElement = document.getElementById(alertId);
        if (alertElement) {
            alertElement.remove();
        }
    }, 5000);
} 