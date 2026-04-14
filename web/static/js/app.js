// Polyant 前端应用

// API 基础地址
const API_BASE = '/api/v1';

// 状态管理
const state = {
    user: null,
    categories: [],
    entries: []
};

// DOM 元素
const elements = {
    searchInput: document.getElementById('searchInput'),
    searchBtn: document.getElementById('searchBtn'),
    loginBtn: document.getElementById('loginBtn'),
    loginModal: document.getElementById('loginModal'),
    closeLoginModal: document.getElementById('closeLoginModal'),
    categoryGrid: document.getElementById('categoryGrid'),
    latestEntries: document.getElementById('latestEntries'),
    popularEntries: document.getElementById('popularEntries')
};

// 初始化
document.addEventListener('DOMContentLoaded', () => {
    initEventListeners();
    loadStats();
    loadCategories();
    loadLatestEntries();
    loadPopularEntries();
});

// 事件监听
function initEventListeners() {
    // 搜索
    elements.searchBtn.addEventListener('click', handleSearch);
    elements.searchInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') handleSearch();
    });

    // 热门标签
    document.querySelectorAll('.tag').forEach(tag => {
        tag.addEventListener('click', () => {
            const tagText = tag.dataset.tag;
            elements.searchInput.value = tagText;
            handleSearch();
        });
    });

    // 登录模态框
    elements.loginBtn.addEventListener('click', () => {
        elements.loginModal.classList.add('active');
    });

    elements.closeLoginModal.addEventListener('click', () => {
        elements.loginModal.classList.remove('active');
    });

    // 切换登录/注册标签
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tab = btn.dataset.tab;
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById(tab + 'Tab').classList.add('active');
        });
    });

    // 登录
    document.getElementById('doLoginBtn').addEventListener('click', handleLogin);

    // 注册
    document.getElementById('doRegisterBtn').addEventListener('click', handleRegister);
}

// 搜索处理
function handleSearch() {
    const query = elements.searchInput.value.trim();
    if (query) {
        window.location.href = `/search?q=${encodeURIComponent(query)}`;
    }
}

// 加载统计数据
async function loadStats() {
    try {
        const response = await fetch(`${API_BASE}/stats`);
        const data = await response.json();
        
        animateNumber('totalEntries', data.total_entries || 0);
        animateNumber('totalUsers', data.total_users || 0);
        animateNumber('totalNodes', data.total_nodes || 0);
        animateNumber('totalRatings', data.total_ratings || 0);
    } catch (error) {
        console.error('加载统计数据失败:', error);
    }
}

// 数字动画
function animateNumber(elementId, target) {
    const element = document.getElementById(elementId);
    const duration = 1000;
    const start = parseInt(element.textContent) || 0;
    const startTime = performance.now();

    function update(currentTime) {
        const elapsed = currentTime - startTime;
        const progress = Math.min(elapsed / duration, 1);
        const current = Math.floor(start + (target - start) * progress);
        element.textContent = current.toLocaleString();

        if (progress < 1) {
            requestAnimationFrame(update);
        }
    }

    requestAnimationFrame(update);
}

// 加载分类
async function loadCategories() {
    try {
        const response = await fetch(`${API_BASE}/categories`);
        const data = await response.json();
        
        state.categories = data.categories || [];
        renderCategories();
    } catch (error) {
        console.error('加载分类失败:', error);
        // 使用默认分类
        renderDefaultCategories();
    }
}

// 渲染分类
function renderCategories() {
    const topLevelCategories = state.categories.filter(c => !c.parent_id);
    
    elements.categoryGrid.innerHTML = topLevelCategories.map(cat => `
        <a href="/categories/${cat.id}" class="category-card">
            <div class="category-icon">${cat.icon || '📁'}</div>
            <div class="category-name">${cat.name}</div>
            <div class="category-count">${cat.entry_count || 0} 条目</div>
        </a>
    `).join('');
}

// 默认分类
function renderDefaultCategories() {
    const defaultCategories = [
        { id: 'tech', icon: '💻', name: '技术', count: 0 },
        { id: 'science', icon: '🔬', name: '科学', count: 0 },
        { id: 'business', icon: '💼', name: '商业', count: 0 },
        { id: 'life', icon: '🏠', name: '生活', count: 0 },
        { id: 'education', icon: '📚', name: '教育', count: 0 },
        { id: 'art', icon: '🎨', name: '艺术', count: 0 },
        { id: 'tools', icon: '🔧', name: '工具', count: 0 },
        { id: 'other', icon: '📁', name: '其他', count: 0 }
    ];

    elements.categoryGrid.innerHTML = defaultCategories.map(cat => `
        <a href="/categories/${cat.id}" class="category-card">
            <div class="category-icon">${cat.icon}</div>
            <div class="category-name">${cat.name}</div>
            <div class="category-count">${cat.count} 条目</div>
        </a>
    `).join('');
}

// 加载最新条目
async function loadLatestEntries() {
    try {
        const response = await fetch(`${API_BASE}/entries?sort=created_at&order=desc&limit=5`);
        const data = await response.json();
        
        state.entries = data.entries || [];
        renderEntries('latestEntries', state.entries);
    } catch (error) {
        console.error('加载最新条目失败:', error);
        elements.latestEntries.innerHTML = '<p class="empty-message">暂无条目</p>';
    }
}

// 加载热门条目
async function loadPopularEntries() {
    try {
        const response = await fetch(`${API_BASE}/entries?sort=score&order=desc&limit=5`);
        const data = await response.json();
        
        renderEntries('popularEntries', data.entries || []);
    } catch (error) {
        console.error('加载热门条目失败:', error);
        elements.popularEntries.innerHTML = '<p class="empty-message">暂无条目</p>';
    }
}

// 渲染条目
function renderEntries(containerId, entries) {
    const container = document.getElementById(containerId);
    
    if (!entries || entries.length === 0) {
        container.innerHTML = '<p class="empty-message">暂无条目</p>';
        return;
    }

    container.innerHTML = entries.map(entry => `
        <div class="entry-card" onclick="location.href='/entries/${entry.id}'">
            <div class="entry-header">
                <div class="entry-title">${escapeHtml(entry.title)}</div>
                <div class="entry-score">★ ${entry.avg_score ? entry.avg_score.toFixed(1) : 'N/A'}</div>
            </div>
            <div class="entry-meta">
                <span><i class="fas fa-user"></i> ${entry.author_name || '匿名'}</span>
                <span><i class="fas fa-folder"></i> ${entry.category || '未分类'}</span>
                <span><i class="fas fa-clock"></i> ${formatDate(entry.created_at)}</span>
            </div>
            <div class="entry-content">${escapeHtml(truncate(entry.content, 200))}</div>
            ${entry.tags && entry.tags.length > 0 ? `
                <div class="entry-tags">
                    ${entry.tags.map(tag => `<span class="entry-tag">${escapeHtml(tag)}</span>`).join('')}
                </div>
            ` : ''}
        </div>
    `).join('');
}

// 登录处理
async function handleLogin() {
    const publicKey = document.getElementById('loginPublicKey').value.trim();
    const signature = document.getElementById('loginSignature').value.trim();

    if (!publicKey || !signature) {
        alert('请填写公钥和签名');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ public_key: publicKey, signature })
        });

        const data = await response.json();

        if (response.ok) {
            state.user = data.user;
            localStorage.setItem('token', data.token);
            elements.loginModal.classList.remove('active');
            updateUserUI();
            alert('登录成功！');
        } else {
            alert(data.error || '登录失败');
        }
    } catch (error) {
        console.error('登录失败:', error);
        alert('登录失败，请稍后重试');
    }
}

// 注册处理
async function handleRegister() {
    const publicKey = document.getElementById('regPublicKey').value.trim();
    const agentName = document.getElementById('regAgentName').value.trim();
    const email = document.getElementById('regEmail').value.trim();

    if (!publicKey || !agentName) {
        alert('请填写公钥和 Agent 名称');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/auth/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                public_key: publicKey, 
                agent_name: agentName,
                email: email || null
            })
        });

        const data = await response.json();

        if (response.ok) {
            alert('注册成功！请登录');
            document.querySelector('[data-tab="login"]').click();
        } else {
            alert(data.error || '注册失败');
        }
    } catch (error) {
        console.error('注册失败:', error);
        alert('注册失败，请稍后重试');
    }
}

// 更新用户 UI
function updateUserUI() {
    if (state.user) {
        elements.loginBtn.innerHTML = `<i class="fas fa-user"></i> ${state.user.agent_name}`;
    }
}

// 工具函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text || '';
    return div.innerHTML;
}

function truncate(text, maxLength) {
    if (!text) return '';
    return text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
}

function formatDate(dateString) {
    if (!dateString) return '未知';
    const date = new Date(dateString);
    const now = new Date();
    const diff = now - date;
    
    if (diff < 60000) return '刚刚';
    if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;
    if (diff < 604800000) return `${Math.floor(diff / 86400000)} 天前`;
    
    return date.toLocaleDateString('zh-CN');
}

// 检查登录状态
function checkAuth() {
    const token = localStorage.getItem('token');
    if (token) {
        fetch(`${API_BASE}/auth/me`, {
            headers: { 'Authorization': `Bearer ${token}` }
        })
        .then(res => res.json())
        .then(data => {
            if (data.user) {
                state.user = data.user;
                updateUserUI();
            }
        })
        .catch(() => {
            localStorage.removeItem('token');
        });
    }
}

// 初始化检查
checkAuth();
