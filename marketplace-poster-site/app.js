document.querySelectorAll('.tab').forEach(tab => tab.addEventListener('click', () => {
  document.querySelectorAll('.tab').forEach(item => item.setAttribute('aria-selected', 'false'));
  document.querySelectorAll('.demo-panel').forEach(panel => panel.classList.remove('active'));
  tab.setAttribute('aria-selected', 'true');
  document.getElementById('panel-' + tab.dataset.panel).classList.add('active');
}));

const posts = document.getElementById('posts');
const manual = document.getElementById('manual');
const setup = document.getElementById('setup');

function updateTime() {
  const weeklyMinutes = Math.max(0, Number(posts.value) * Number(manual.value) - Number(setup.value));
  const weeklyHours = weeklyMinutes / 60;
  document.getElementById('postsOut').value = posts.value;
  document.getElementById('manualOut').value = manual.value;
  document.getElementById('setupOut').value = setup.value + ' דקות';
  document.getElementById('weekSaved').textContent = weeklyHours.toFixed(1) + ' שעות';
  document.getElementById('monthSaved').textContent = (weeklyHours * 4).toFixed(1) + ' שעות';
}

[posts, manual, setup].forEach(input => input.addEventListener('input', updateTime));

const leads = document.getElementById('leads');
const exposure = document.getElementById('exposure');

function updateSales() {
  const estimate = Math.max(0, Number(leads.value) * (1 + Number(exposure.value) / 100));
  document.getElementById('leadResult').textContent = 'כ-' + Math.round(estimate) + ' פניות';
}

[leads, exposure].forEach(input => input.addEventListener('input', updateSales));

document.querySelectorAll('[data-billing]').forEach(button => button.addEventListener('click', () => {
  const annual = button.dataset.billing === 'annual';
  document.querySelectorAll('[data-billing]').forEach(item => item.classList.toggle('active', item === button));
  document.querySelectorAll('.price strong').forEach(price => {
    price.textContent = '₪' + (annual ? price.dataset.annual : price.dataset.monthly);
  });
  document.querySelectorAll('.annual-detail').forEach(detail => {
    detail.textContent = annual ? detail.dataset.annualCopy : '';
  });
}));
