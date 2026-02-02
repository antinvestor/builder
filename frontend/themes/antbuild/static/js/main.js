// ant.build Main JavaScript

// Mobile menu toggle
function toggleMenu() {
  const nav = document.getElementById('main-nav');
  nav.classList.toggle('active');
}

// Smooth scroll for anchor links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
  anchor.addEventListener('click', function (e) {
    e.preventDefault();
    const target = document.querySelector(this.getAttribute('href'));
    if (target) {
      target.scrollIntoView({
        behavior: 'smooth',
        block: 'start'
      });
    }
  });
});

// Header scroll effect
let lastScroll = 0;
const header = document.querySelector('.header');

window.addEventListener('scroll', () => {
  const currentScroll = window.pageYOffset;

  if (currentScroll > 100) {
    header.style.background = 'rgba(10, 10, 15, 0.95)';
  } else {
    header.style.background = 'rgba(10, 10, 15, 0.8)';
  }

  lastScroll = currentScroll;
});

// Intersection Observer for animations
const observerOptions = {
  root: null,
  rootMargin: '0px',
  threshold: 0.1
};

const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      entry.target.classList.add('animate-in');
      observer.unobserve(entry.target);
    }
  });
}, observerOptions);

document.querySelectorAll('.feature-card, .pricing-card, .blog-card, .step').forEach(el => {
  el.style.opacity = '0';
  observer.observe(el);
});

// Terminal typing animation
function typeWriter(element, text, speed = 50) {
  let i = 0;
  element.innerHTML = '';

  function type() {
    if (i < text.length) {
      element.innerHTML += text.charAt(i);
      i++;
      setTimeout(type, speed);
    }
  }

  type();
}

// Initialize terminal animations
document.querySelectorAll('.terminal-command').forEach((el, index) => {
  const text = el.textContent;
  el.textContent = '';
  setTimeout(() => {
    typeWriter(el, text, 30);
  }, index * 1000);
});

// Copy code block functionality
document.querySelectorAll('pre code').forEach(codeBlock => {
  const wrapper = codeBlock.parentElement;
  const button = document.createElement('button');
  button.className = 'copy-btn';
  button.textContent = 'Copy';
  button.style.cssText = `
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    padding: 0.25rem 0.75rem;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--text-muted);
    font-size: 0.75rem;
    cursor: pointer;
    transition: all 0.2s;
  `;

  wrapper.style.position = 'relative';
  wrapper.appendChild(button);

  button.addEventListener('click', async () => {
    await navigator.clipboard.writeText(codeBlock.textContent);
    button.textContent = 'Copied!';
    button.style.color = 'var(--accent-green)';
    setTimeout(() => {
      button.textContent = 'Copy';
      button.style.color = 'var(--text-muted)';
    }, 2000);
  });
});

// Form validation helper
function validateEmail(email) {
  const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return re.test(email);
}

// Show toast notification
function showToast(message, type = 'info') {
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  toast.style.cssText = `
    position: fixed;
    bottom: 2rem;
    right: 2rem;
    padding: 1rem 1.5rem;
    background: var(--bg-card);
    border: 1px solid ${type === 'success' ? 'var(--accent-green)' : type === 'error' ? '#ef4444' : 'var(--border)'};
    border-radius: 0.75rem;
    color: var(--text-primary);
    font-size: 0.9375rem;
    box-shadow: var(--shadow-lg);
    z-index: 1000;
    animation: slideIn 0.3s ease;
  `;

  document.body.appendChild(toast);

  setTimeout(() => {
    toast.style.animation = 'slideOut 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 3000);
}

// Add slide animations
const style = document.createElement('style');
style.textContent = `
  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  @keyframes slideOut {
    from { transform: translateX(0); opacity: 1; }
    to { transform: translateX(100%); opacity: 0; }
  }
`;
document.head.appendChild(style);

// Expose functions globally
window.toggleMenu = toggleMenu;
window.showToast = showToast;
window.validateEmail = validateEmail;
