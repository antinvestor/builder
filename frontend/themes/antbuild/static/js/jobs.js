// ant.build Jobs Search
// Client-side job search and filtering

// Sample job data (in production, this would come from an API)
const jobsData = [
  {
    id: 1,
    title: "Senior Go Developer",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Engineering",
    salary: "$150k - $200k",
    posted: "2026-01-28",
    description: "Join our core platform team building the next generation of AI-powered development tools.",
    requirements: ["5+ years Go experience", "Distributed systems", "Kubernetes", "Event-driven architecture"],
    tags: ["Go", "Kubernetes", "gRPC", "PostgreSQL"]
  },
  {
    id: 2,
    title: "Full Stack Engineer",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Engineering",
    salary: "$130k - $180k",
    posted: "2026-01-25",
    description: "Build beautiful, performant user interfaces for our AI code generation platform.",
    requirements: ["React/Vue expertise", "TypeScript", "Node.js", "CSS/Tailwind"],
    tags: ["React", "TypeScript", "Node.js", "Hugo"]
  },
  {
    id: 3,
    title: "ML/AI Engineer",
    company: "Antinvestor",
    location: "Remote / San Francisco",
    type: "Full-time",
    department: "AI/ML",
    salary: "$180k - $250k",
    posted: "2026-01-20",
    description: "Improve our AI code generation pipeline and develop new LLM orchestration strategies.",
    requirements: ["LLM fine-tuning experience", "Python", "BAML/Langchain", "Production ML systems"],
    tags: ["Python", "LLM", "BAML", "Claude", "GPT"]
  },
  {
    id: 4,
    title: "DevOps Engineer",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Infrastructure",
    salary: "$140k - $190k",
    posted: "2026-01-18",
    description: "Scale our infrastructure and ensure reliability across our distributed platform.",
    requirements: ["Kubernetes expertise", "Terraform", "AWS/GCP", "Observability stack"],
    tags: ["Kubernetes", "Terraform", "Prometheus", "GitOps"]
  },
  {
    id: 5,
    title: "Product Designer",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Design",
    salary: "$120k - $170k",
    posted: "2026-01-15",
    description: "Design intuitive experiences for developers using AI-powered tools.",
    requirements: ["Figma expertise", "Design systems", "User research", "Developer tools experience"],
    tags: ["Figma", "UI/UX", "Design Systems", "Prototyping"]
  },
  {
    id: 6,
    title: "Technical Writer",
    company: "Antinvestor",
    location: "Remote",
    type: "Contract",
    department: "Documentation",
    salary: "$80k - $120k",
    posted: "2026-01-10",
    description: "Create clear, comprehensive documentation for our platform and APIs.",
    requirements: ["Technical writing experience", "API documentation", "Developer audience", "Git/Markdown"],
    tags: ["Documentation", "API", "Markdown", "Developer Experience"]
  },
  {
    id: 7,
    title: "Security Engineer",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Security",
    salary: "$160k - $220k",
    posted: "2026-01-08",
    description: "Secure our platform and ensure enterprise-grade protection for customer code.",
    requirements: ["Application security", "Container security", "Penetration testing", "Compliance (SOC2)"],
    tags: ["Security", "Vault", "mTLS", "SOC2"]
  },
  {
    id: 8,
    title: "Developer Advocate",
    company: "Antinvestor",
    location: "Remote",
    type: "Full-time",
    department: "Developer Relations",
    salary: "$130k - $175k",
    posted: "2026-01-05",
    description: "Help developers succeed with our platform through content, community, and support.",
    requirements: ["Software development background", "Public speaking", "Content creation", "Community building"],
    tags: ["DevRel", "Content", "Community", "Speaking"]
  }
];

let filteredJobs = [...jobsData];

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  renderJobs(jobsData);
  initFilters();
});

function initFilters() {
  // Search input
  const searchInput = document.getElementById('job-search');
  if (searchInput) {
    searchInput.addEventListener('input', debounce(filterJobs, 300));
  }

  // Department filter
  const deptFilter = document.getElementById('department-filter');
  if (deptFilter) {
    // Populate departments
    const departments = [...new Set(jobsData.map(j => j.department))];
    departments.forEach(dept => {
      const option = document.createElement('option');
      option.value = dept;
      option.textContent = dept;
      deptFilter.appendChild(option);
    });
    deptFilter.addEventListener('change', filterJobs);
  }

  // Type filter
  const typeFilter = document.getElementById('type-filter');
  if (typeFilter) {
    typeFilter.addEventListener('change', filterJobs);
  }

  // Location filter
  const locationFilter = document.getElementById('location-filter');
  if (locationFilter) {
    const locations = [...new Set(jobsData.map(j => j.location))];
    locations.forEach(loc => {
      const option = document.createElement('option');
      option.value = loc;
      option.textContent = loc;
      locationFilter.appendChild(option);
    });
    locationFilter.addEventListener('change', filterJobs);
  }
}

function filterJobs() {
  const search = document.getElementById('job-search')?.value.toLowerCase() || '';
  const department = document.getElementById('department-filter')?.value || '';
  const type = document.getElementById('type-filter')?.value || '';
  const location = document.getElementById('location-filter')?.value || '';

  filteredJobs = jobsData.filter(job => {
    // Search filter
    const searchMatch = !search ||
      job.title.toLowerCase().includes(search) ||
      job.description.toLowerCase().includes(search) ||
      job.tags.some(tag => tag.toLowerCase().includes(search));

    // Department filter
    const deptMatch = !department || job.department === department;

    // Type filter
    const typeMatch = !type || job.type === type;

    // Location filter
    const locationMatch = !location || job.location === location;

    return searchMatch && deptMatch && typeMatch && locationMatch;
  });

  renderJobs(filteredJobs);
}

function renderJobs(jobs) {
  const container = document.getElementById('jobs-list');
  if (!container) return;

  if (jobs.length === 0) {
    container.innerHTML = `
      <div class="no-jobs">
        <h3>No jobs found</h3>
        <p>Try adjusting your search criteria or check back later for new opportunities.</p>
      </div>
    `;
    return;
  }

  container.innerHTML = jobs.map(job => `
    <article class="job-card" data-job-id="${job.id}">
      <div class="job-info">
        <h3><a href="/jobs/${job.id}/">${job.title}</a></h3>
        <div class="job-meta">
          <span>📍 ${job.location}</span>
          <span>💼 ${job.type}</span>
          <span>🏢 ${job.department}</span>
          <span>💰 ${job.salary}</span>
        </div>
        <p style="margin-top: 0.75rem; font-size: 0.9375rem; color: var(--text-secondary);">
          ${job.description}
        </p>
        <div class="job-tags">
          ${job.tags.map(tag => `<span class="job-tag">${tag}</span>`).join('')}
        </div>
      </div>
      <div class="job-actions">
        <a href="/jobs/${job.id}/" class="btn btn-primary btn-sm">Apply Now</a>
      </div>
    </article>
  `).join('');

  // Update results count
  const countEl = document.getElementById('jobs-count');
  if (countEl) {
    countEl.textContent = `${jobs.length} position${jobs.length !== 1 ? 's' : ''} available`;
  }
}

function debounce(func, wait) {
  let timeout;
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
}

// Format relative date
function formatDate(dateString) {
  const date = new Date(dateString);
  const now = new Date();
  const diffDays = Math.floor((now - date) / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 7) return `${diffDays} days ago`;
  if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
  return `${Math.floor(diffDays / 30)} months ago`;
}
