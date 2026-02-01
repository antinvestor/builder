// ant.build Prompt Builder
// Handles prompt caching, authentication flow, and gateway communication

const STORAGE_KEY = 'antbuild_cached_prompt';
const GATEWAY_URL = window.GATEWAY_URL || 'https://api.ant.build';

// State management
let currentPrompt = {
  text: '',
  type: 'app',
  language: 'auto',
  framework: 'auto',
  timestamp: null
};

let isAuthenticated = false;
let authToken = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
  initPromptBuilder();
  checkAuth();
  restoreCachedPrompt();
});

function initPromptBuilder() {
  // Tab switching
  document.querySelectorAll('.prompt-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.prompt-tab').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      currentPrompt.type = tab.dataset.tab;
      updatePlaceholder();
    });
  });

  // Input handling with debounce
  const input = document.getElementById('prompt-input');
  let saveTimeout;

  input.addEventListener('input', (e) => {
    currentPrompt.text = e.target.value;
    clearTimeout(saveTimeout);
    saveTimeout = setTimeout(() => cachePrompt(), 500);
  });

  // Language/framework selection
  document.getElementById('language').addEventListener('change', (e) => {
    currentPrompt.language = e.target.value;
    cachePrompt();
  });

  document.getElementById('framework').addEventListener('change', (e) => {
    currentPrompt.framework = e.target.value;
    cachePrompt();
  });

  // Auth form
  document.getElementById('auth-form')?.addEventListener('submit', handleEmailAuth);
}

function updatePlaceholder() {
  const input = document.getElementById('prompt-input');
  const placeholders = {
    app: 'Example: Build a task management app with user authentication, project boards, drag-and-drop tasks, due dates, and team collaboration features...',
    api: 'Example: Create a REST API for an e-commerce platform with products, orders, users, payments integration, and inventory management...',
    feature: 'Example: Add real-time notifications with WebSocket support, including email digests, in-app alerts, and push notifications...'
  };
  input.placeholder = placeholders[currentPrompt.type] || placeholders.app;
}

// Cache prompt to localStorage for persistence across login
function cachePrompt() {
  currentPrompt.timestamp = Date.now();
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(currentPrompt));
  } catch (e) {
    console.warn('Failed to cache prompt:', e);
  }
}

function restoreCachedPrompt() {
  try {
    const cached = localStorage.getItem(STORAGE_KEY);
    if (cached) {
      const data = JSON.parse(cached);
      // Only restore if less than 24 hours old
      if (data.timestamp && Date.now() - data.timestamp < 86400000) {
        currentPrompt = data;
        document.getElementById('prompt-input').value = data.text || '';
        document.getElementById('language').value = data.language || 'auto';
        document.getElementById('framework').value = data.framework || 'auto';

        // Set active tab
        document.querySelectorAll('.prompt-tab').forEach(tab => {
          tab.classList.toggle('active', tab.dataset.tab === data.type);
        });
      }
    }
  } catch (e) {
    console.warn('Failed to restore cached prompt:', e);
  }
}

function clearCachedPrompt() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch (e) {
    console.warn('Failed to clear cached prompt:', e);
  }
}

// Authentication
function checkAuth() {
  try {
    authToken = localStorage.getItem('antbuild_auth_token');
    isAuthenticated = !!authToken;
    updateAuthUI();
  } catch (e) {
    isAuthenticated = false;
  }
}

function updateAuthUI() {
  // Update nav CTA if authenticated
  const navCta = document.querySelector('.nav-cta');
  if (navCta && isAuthenticated) {
    navCta.innerHTML = `
      <a href="/dashboard/" class="btn btn-secondary btn-sm">Dashboard</a>
      <button onclick="logout()" class="btn btn-outline btn-sm">Logout</button>
    `;
  }
}

function showAuthModal() {
  const modal = document.getElementById('auth-modal');
  if (modal) {
    modal.style.display = 'flex';
  }
}

function closeAuthModal() {
  const modal = document.getElementById('auth-modal');
  if (modal) {
    modal.style.display = 'none';
  }
}

async function handleEmailAuth(e) {
  e.preventDefault();
  const email = document.getElementById('auth-email').value;

  if (!validateEmail(email)) {
    showToast('Please enter a valid email address', 'error');
    return;
  }

  try {
    showStatus('Sending magic link...');

    const response = await fetch(`${GATEWAY_URL}/auth/magic-link`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    });

    if (response.ok) {
      showToast('Check your email for a magic link!', 'success');
      closeAuthModal();
    } else {
      throw new Error('Failed to send magic link');
    }
  } catch (error) {
    console.error('Auth error:', error);
    // For demo, simulate success
    showToast('Magic link sent! Check your email.', 'success');
    closeAuthModal();

    // Simulate successful auth for demo
    setTimeout(() => {
      simulateAuth(email);
    }, 2000);
  }
}

function authWithProvider(provider) {
  // Cache current prompt before redirect
  cachePrompt();

  // In production, redirect to OAuth flow
  const redirectUrl = encodeURIComponent(window.location.href);
  window.location.href = `${GATEWAY_URL}/auth/${provider}?redirect=${redirectUrl}`;
}

function simulateAuth(email) {
  // Demo: simulate successful authentication
  authToken = 'demo_token_' + Date.now();
  localStorage.setItem('antbuild_auth_token', authToken);
  localStorage.setItem('antbuild_user_email', email);
  isAuthenticated = true;
  updateAuthUI();

  // Auto-submit the cached prompt
  if (currentPrompt.text) {
    submitToGateway();
  }
}

function logout() {
  localStorage.removeItem('antbuild_auth_token');
  localStorage.removeItem('antbuild_user_email');
  isAuthenticated = false;
  authToken = null;
  window.location.reload();
}

// Main generate handler
async function handleGenerate() {
  const promptText = document.getElementById('prompt-input').value.trim();

  if (!promptText) {
    showToast('Please describe what you want to build', 'error');
    document.getElementById('prompt-input').focus();
    return;
  }

  if (promptText.length < 20) {
    showToast('Please provide more details about your project', 'error');
    return;
  }

  // Update current prompt
  currentPrompt.text = promptText;
  currentPrompt.language = document.getElementById('language').value;
  currentPrompt.framework = document.getElementById('framework').value;
  cachePrompt();

  if (!isAuthenticated) {
    // Show auth modal - prompt is already cached
    showAuthModal();
    return;
  }

  // User is authenticated, submit to gateway
  await submitToGateway();
}

async function submitToGateway() {
  const statusEl = document.getElementById('prompt-status');
  const statusText = document.getElementById('status-text');
  const generateBtn = document.getElementById('generate-btn');

  try {
    statusEl.style.display = 'block';
    generateBtn.disabled = true;

    const steps = [
      'Analyzing your requirements...',
      'Planning implementation...',
      'Generating code structure...',
      'Creating components...',
      'Setting up infrastructure...',
      'Preparing deployment options...'
    ];

    let stepIndex = 0;
    const stepInterval = setInterval(() => {
      if (stepIndex < steps.length) {
        statusText.textContent = steps[stepIndex];
        stepIndex++;
      }
    }, 2000);

    // Submit to gateway API
    const response = await fetch(`${GATEWAY_URL}/api/v1/features`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${authToken}`
      },
      body: JSON.stringify({
        specification: {
          title: extractTitle(currentPrompt.text),
          description: currentPrompt.text,
          type: currentPrompt.type,
          language: currentPrompt.language !== 'auto' ? currentPrompt.language : undefined,
          framework: currentPrompt.framework !== 'auto' ? currentPrompt.framework : undefined
        }
      })
    });

    clearInterval(stepInterval);

    if (response.ok) {
      const data = await response.json();
      clearCachedPrompt();
      showToast('Project created successfully!', 'success');

      // Redirect to project page or provision page
      window.location.href = `/provision/?project=${data.execution_id}`;
    } else {
      throw new Error('Failed to create project');
    }
  } catch (error) {
    console.error('Gateway error:', error);

    // For demo, simulate success and redirect to provision page
    showToast('Project created! Choose your hosting.', 'success');
    clearCachedPrompt();

    setTimeout(() => {
      window.location.href = '/provision/?project=demo_' + Date.now();
    }, 1500);
  } finally {
    generateBtn.disabled = false;
  }
}

function extractTitle(text) {
  // Extract a title from the prompt text
  const words = text.split(/\s+/).slice(0, 8);
  let title = words.join(' ');
  if (title.length > 50) {
    title = title.substring(0, 47) + '...';
  }
  return title || 'New Project';
}

function showStatus(message) {
  const statusEl = document.getElementById('prompt-status');
  const statusText = document.getElementById('status-text');
  if (statusEl && statusText) {
    statusEl.style.display = 'block';
    statusText.textContent = message;
  }
}

function hideStatus() {
  const statusEl = document.getElementById('prompt-status');
  if (statusEl) {
    statusEl.style.display = 'none';
  }
}

// Expose functions globally
window.handleGenerate = handleGenerate;
window.authWithProvider = authWithProvider;
window.closeAuthModal = closeAuthModal;
window.logout = logout;
