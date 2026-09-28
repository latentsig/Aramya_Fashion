const products = [
  {name:'The Heirloom Saree',category:'sarees',label:'Sarees',img:'saree1.jpg',price:8900},
  {name:'Saanjh Silk Saree',category:'sarees',label:'Sarees',img:'saree3.jpg',price:12500},
  {name:'Rosewater Drapes',category:'sarees',label:'Sarees',img:'saree2.jpg',price:9800},
  {name:'Temple Garden Saree',category:'sarees',label:'Sarees',img:'saree4.jpg',price:14200},
  {name:'The Meera Edit',category:'sarees',label:'Sarees',img:'saree5.jpg',price:11900},
  {name:'Gulabi Lehenga',category:'bridal',label:'Bridal & lehengas',img:'lehanga1.jpg',price:28000},
  {name:'The Noor Edit',category:'bridal',label:'Bridal & lehengas',img:'lehanga4.jpg',price:34500},
  {name:'Gul Mahal Lehenga',category:'bridal',label:'Bridal & lehengas',img:'lehanga2.jpg',price:32000},
  {name:'Aabha Embroidered Lehenga',category:'bridal',label:'Bridal & lehengas',img:'lehanga3.jpg',price:29500},
  {name:'The Sunehri Set',category:'bridal',label:'Bridal & lehengas',img:'lehanga5.jpg',price:36000},
  {name:'Rani Sharara Set',category:'occasion',label:'Occasionwear',img:'sharara1.avif',price:16900},
  {name:'Evening in Rose',category:'occasion',label:'Occasionwear',img:'gown2.webp',price:19500},
  {name:'The Celebration Gown',category:'occasion',label:'Occasionwear',img:'gown4.webp',price:24000},
  {name:'Festive Half Saree',category:'occasion',label:'Occasionwear',img:'halfsaree2.jpg',price:15500},
  {name:'Mehfil Sharara',category:'occasion',label:'Occasionwear',img:'sharara2.webp',price:17500},
  {name:'Gulnaar Sharara Set',category:'occasion',label:'Occasionwear',img:'sharara3.webp',price:18900},
  {name:'The Noor Sharara',category:'occasion',label:'Occasionwear',img:'sharara4.webp',price:16500},
  {name:'Celebration in Bloom',category:'occasion',label:'Occasionwear',img:'sharara5.webp',price:20000},
  {name:'Soft Gold Gown',category:'occasion',label:'Occasionwear',img:'gown3.webp',price:22500},
  {name:'The Saanvi Gown',category:'occasion',label:'Occasionwear',img:'gown6.webp',price:21000},
  {name:'Phool Half Saree',category:'occasion',label:'Occasionwear',img:'halfsaree1.jpg',price:14500},
  {name:'Madhuri Half Saree',category:'occasion',label:'Occasionwear',img:'halfsaree3.jpg',price:16000},
  {name:'The Lilac Half Saree',category:'occasion',label:'Occasionwear',img:'halfsaree4.jpg',price:15900},
  {name:'Sundari Half Saree',category:'occasion',label:'Occasionwear',img:'halfsaree5.jpg',price:15500},
  {name:'Modern Bandhgala',category:'menswear',label:'Menswear',img:'menblazer2.jpg',price:18500},
  {name:'The Ivory Bandhgala',category:'menswear',label:'Menswear',img:'menblazer1.jpg',price:20500},
  {name:'Aramya Evening Blazer',category:'menswear',label:'Menswear',img:'menblazer3.jpg',price:17900},
  {name:'The Saffron Blazer',category:'menswear',label:'Menswear',img:'menblazer4.jpg',price:19500},
  {name:'Classic Celebration Blazer',category:'menswear',label:'Menswear',img:'menblazer5.jpg',price:18200},
  {name:'The Aramya Kurtha',category:'menswear',label:'Menswear',img:'menkurtha3.jpg',price:7900},
  {name:'The Festive Kurtha',category:'menswear',label:'Menswear',img:'menkurtha1.jpg',price:7500},
  {name:'Ivory Occasion Kurtha',category:'menswear',img:'menkurtha2.jpg',label:'Menswear',price:8200},
  {name:'The Heritage Kurtha',category:'menswear',label:'Menswear',img:'menkurtha4.jpg',price:8900},
  {name:'Sage Celebration Kurtha',category:'menswear',label:'Menswear',img:'menkurtha5.jpg',price:7900},
  {name:'Little Celebration',category:'kids',label:'Little ones',img:'kidzgown2.jpg',price:4500},
  {name:'Tiny Festive Set',category:'kids',label:'Little ones',img:'kidzgown4.jpg',price:5200},
  {name:'Little Rose Gown',category:'kids',label:'Little ones',img:'kidzgown1.jpg',price:4200},
  {name:'A Little Celebration',category:'kids',label:'Little ones',img:'kidzgown3.jpg',price:4800},
  {name:'Tiny Twirl Gown',category:'kids',label:'Little ones',img:'kidzgown5.jpg',price:5100}
];
const categories = [
  {id:'sarees',title:'Sarees',subtitle:'A drape for every celebration',image:'saree1.jpg'},
  {id:'bridal',title:'Bridal & Lehengas',subtitle:'For the day you will always remember',image:'bridal lehanga.jpg'},
  {id:'occasion',title:'Occasionwear',subtitle:'Festive sets, gowns and shararas',image:'sharara1.avif'},
  {id:'menswear',title:'Menswear',subtitle:'Considered festive dressing',image:'menblazer2.jpg'},
  {id:'kids',title:'Little Ones',subtitle:'A little something for their big day',image:'kidzgown2.jpg'}
];
const money = amount => `${String.fromCharCode(8377)} ${new Intl.NumberFormat('en-IN').format(amount)}`;
const grid = document.querySelector('#product-grid');
const categoryGrid = document.querySelector('#category-grid');
const activeCategory = document.body.dataset.category;
let toastTimer;
let cart = {};
try { cart = JSON.parse(localStorage.getItem('aramya-cart') || '{}') || {}; } catch { cart = {}; }

function productCard(p) {
  return `<article class="product-card"><div class="product-image"><img src="image assests/${p.img}" alt="${p.name}" loading="lazy"><span class="product-tag">${p.label}</span><button class="quick-add" type="button" data-add="${p.name}">Add to bag <span>+</span></button></div><div class="product-info"><div><span class="product-name">${p.name}</span><span class="product-category">Made to order · Custom fit available</span></div><span class="product-price">${money(p.price)}</span></div></article>`;
}
function renderProducts() {
  if (!grid) return;
  const list = products.filter(p => p.category === activeCategory);
  grid.innerHTML = list.map(productCard).join('');
  observeReveals(grid.querySelectorAll('.product-card'));
}
function renderCategories() {
  if (!categoryGrid) return;
  categoryGrid.innerHTML = categories.map((c, index) => `<a class="category-card reveal" style="--reveal-delay:${index * 65}ms" href="${c.id}.html"><div class="category-image"><img src="image assests/${c.image}" alt="Explore ${c.title}" loading="lazy"><span class="category-count">${products.filter(p => p.category === c.id).length} styles</span></div><div class="category-card-copy"><div><h3>${c.title}</h3><p>${c.subtitle}</p></div><span class="category-arrow">↗</span></div></a>`).join('');
  observeReveals(categoryGrid.querySelectorAll('.category-card'));
}
function notify(message) {
  const el = document.querySelector('#toast');
  if (!el) return;
  el.textContent = message; el.classList.add('show'); clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove('show'), 2300);
}
const drawer = document.createElement('div');
drawer.className = 'cart-layer';
drawer.innerHTML = `<button class="cart-backdrop" type="button" aria-label="Close bag"></button><aside class="cart-drawer" role="dialog" aria-modal="true" aria-labelledby="cart-title"><div class="cart-head"><div><p class="eyebrow">A LITTLE SOMETHING</p><h2 id="cart-title">Your bag</h2></div><button class="cart-close" type="button" aria-label="Close bag">×</button></div><div class="cart-items"></div><div class="cart-bottom"><div class="cart-total"><span>Subtotal</span><strong></strong></div><p>Made to order. Fit, delivery and final pricing are confirmed by our studio.</p><a class="button button-red cart-order" href="checkout.html">Continue to checkout <span>↗</span></a></div></aside>`;
document.body.append(drawer);
function saveCart() { localStorage.setItem('aramya-cart', JSON.stringify(cart)); }
function cartCount() { return Object.values(cart).reduce((sum, quantity) => sum + quantity, 0); }
function renderCart() {
  const count = cartCount();
  document.querySelectorAll('.bag-count').forEach(el => el.textContent = count);
  const items = Object.entries(cart).map(([name, quantity]) => ({product:products.find(p => p.name === name), quantity})).filter(item => item.product);
  drawer.querySelector('.cart-items').innerHTML = items.length ? items.map(({product:p, quantity}) => `<div class="cart-item"><img src="image assests/${p.img}" alt=""><div class="cart-item-copy"><strong>${p.name}</strong><span>${money(p.price)}</span><div class="quantity-control"><button type="button" data-qty="-1" data-name="${p.name}" aria-label="Remove one ${p.name}">−</button><span>${quantity}</span><button type="button" data-qty="1" data-name="${p.name}" aria-label="Add one ${p.name}">+</button></div></div><button class="cart-remove" type="button" data-remove="${p.name}" aria-label="Remove ${p.name}">Remove</button></div>`).join('') : `<div class="cart-empty"><span>♡</span><h3>Your bag is waiting.</h3><p>Explore the collections and save a piece for your next celebration.</p><a class="text-link" href="index.html#collections">Explore collections ↗</a></div>`;
  const total = items.reduce((sum, {product, quantity}) => sum + product.price * quantity, 0);
  drawer.querySelector('.cart-total strong').textContent = money(total);
  drawer.querySelector('.cart-bottom').hidden = !items.length;
  renderCheckout();
}
function renderCheckout() {
  const list = document.querySelector('#checkout-items');
  if (!list) return;
  const items = Object.entries(cart).map(([name, quantity]) => ({product:products.find(p => p.name === name), quantity})).filter(item => item.product);
  list.innerHTML = items.map(({product:p, quantity}) => `<div class="checkout-item"><img src="image assests/${p.img}" alt=""><div><strong>${p.name}</strong><span>${quantity} × ${money(p.price)}</span></div><b>${money(p.price * quantity)}</b></div>`).join('');
  document.querySelector('#checkout-subtotal').textContent = money(items.reduce((sum, {product, quantity}) => sum + product.price * quantity, 0));
  document.querySelector('.checkout-layout').hidden = !items.length;
  document.querySelector('#checkout-empty').hidden = items.length > 0;
}
function openCart() { renderCart(); drawer.classList.add('is-open'); document.body.classList.add('overlay-open'); drawer.querySelector('.cart-close').focus(); }
function closeCart() { drawer.classList.remove('is-open'); document.body.classList.remove('overlay-open'); }
let revealObserver;
renderCategories(); renderProducts(); renderCart();
document.body.classList.add('js-ready');
function observeReveals(elements) {
  if (!('IntersectionObserver' in window)) { elements.forEach(el => el.classList.add('is-visible')); return; }
  if (!revealObserver) revealObserver = new IntersectionObserver(entries => entries.forEach(entry => { if (entry.isIntersecting) { entry.target.classList.add('is-visible'); revealObserver.unobserve(entry.target); } }), {threshold:.12, rootMargin:'0px 0px -35px 0px'});
  elements.forEach(el => { el.classList.add('reveal'); revealObserver.observe(el); });
}
observeReveals(document.querySelectorAll('.intro,.section-heading,.collections-intro,.custom-image,.custom-copy,.studio-location,footer'));
document.querySelectorAll('.bag-button').forEach(button => button.addEventListener('click', openCart));
document.querySelector('.cart-close').addEventListener('click', closeCart);
drawer.querySelector('.cart-backdrop').addEventListener('click', closeCart);
document.addEventListener('keydown', event => { if (event.key === 'Escape') { closeCart(); document.querySelector('#appointment-dialog')?.close(); } });
document.addEventListener('click', event => {
  const add = event.target.closest('[data-add]');
  if (add) { const name = add.dataset.add; cart[name] = (cart[name] || 0) + 1; saveCart(); renderCart(); notify(`${name} added to your bag`); return; }
  const qty = event.target.closest('[data-qty]');
  if (qty) { const name = qty.dataset.name; cart[name] = (cart[name] || 0) + Number(qty.dataset.qty); if (cart[name] <= 0) delete cart[name]; saveCart(); renderCart(); return; }
  const remove = event.target.closest('[data-remove]');
  if (remove) { delete cart[remove.dataset.remove]; saveCart(); renderCart(); return; }
  const appointmentLink = event.target.closest('a[href="#appointment"], .appointment-trigger');
  if (appointmentLink) { event.preventDefault(); document.body.classList.add('overlay-open'); document.querySelector('#appointment-dialog')?.showModal(); document.querySelector('#appointment-dialog input[name="name"]')?.focus(); return; }
});
const menuToggle = document.querySelector('.menu-toggle');
menuToggle?.addEventListener('click', event => { const nav = document.querySelector('.nav-left'); const open = nav.classList.toggle('open'); event.currentTarget.setAttribute('aria-expanded', open); });
document.querySelectorAll('.nav-left a').forEach(link => link.addEventListener('click', () => document.querySelector('.nav-left').classList.remove('open')));
const appointmentDialog = document.querySelector('#appointment-dialog');
appointmentDialog?.querySelector('.dialog-close').addEventListener('click', () => appointmentDialog.close());
appointmentDialog?.addEventListener('click', event => { if (event.target === appointmentDialog) appointmentDialog.close(); });
appointmentDialog?.addEventListener('close', () => document.body.classList.remove('overlay-open'));
appointmentDialog?.addEventListener('cancel', () => document.body.classList.remove('overlay-open'));
document.querySelectorAll('.appointment-trigger').forEach(link => link.addEventListener('click', () => document.body.classList.add('overlay-open')));
async function sendToStudio(fields, subject) {
  const payload = Object.fromEntries(fields.entries());
  payload._subject = subject;
  payload._template = 'table';
  const response = await fetch('https://formsubmit.co/ajax/hello@aramyafashionhouse.com', {
    method: 'POST',
    headers: {'Content-Type':'application/json','Accept':'application/json'},
    body: JSON.stringify(payload)
  });
  const result = await response.json();
  if (!response.ok || result.success === false || result.success === 'false') {
    throw new Error(result.message || 'The request could not be sent. Please try again.');
  }
  return result;
}
function attachDirectSubmit(form, status, subject, successText, afterSuccess) {
  if (!form) return;
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (!form.reportValidity()) return;
    const button = form.querySelector('[type="submit"]');
    const originalText = button.textContent;
    button.disabled = true;
    button.textContent = 'Sending…';
    status.textContent = 'Sending your details to the Aramya studio…';
    try {
      await sendToStudio(new FormData(form), subject);
      status.textContent = successText;
      button.textContent = 'Sent';
      if (afterSuccess) afterSuccess();
    } catch (error) {
      status.textContent = 'We could not send this just now. Please try again in a moment.';
      button.disabled = false;
      button.textContent = originalText;
    }
  });
}
attachDirectSubmit(
  document.querySelector('#appointment-form'),
  document.querySelector('#form-note'),
  'New Aramya studio appointment request',
  'Your appointment request has been sent to the studio. We will contact you to arrange a time.'
);
const checkoutForm = document.querySelector('#checkout-form');
checkoutForm?.addEventListener('submit', async event => {
  event.preventDefault();
  if (!checkoutForm.reportValidity()) return;
  const items = Object.entries(cart).map(([name, quantity]) => {
    const product = products.find(p => p.name === name);
    return product ? `${quantity} x ${name} — ${money(product.price * quantity)}` : '';
  }).filter(Boolean);
  if (!items.length) { renderCheckout(); return; }
  const total = Object.entries(cart).reduce((sum,[name,quantity]) => sum + (products.find(p => p.name === name)?.price || 0) * quantity, 0);
  const fields = new FormData(checkoutForm);
  fields.set('Order items', items.join('\n'));
  fields.set('Estimated subtotal', money(total));
  fields.set('Order location', 'Aramya Fashion House, Ankola, Uttara Kannada, Karnataka');
  const button = checkoutForm.querySelector('[type="submit"]');
  const originalText = button.textContent;
  const status = document.querySelector('#checkout-feedback');
  button.disabled = true;
  button.textContent = 'Sending…';
  status.textContent = 'Sending your order request to the Aramya studio…';
  try {
    await sendToStudio(fields, 'New Aramya order request');
    status.textContent = 'Your order request has been sent to the studio. We will contact you to confirm fit, delivery and payment.';
    button.textContent = 'Request sent';
    cart = {};
    saveCart();
    renderCart();
    document.querySelector('#order-confirmation')?.showModal();
  } catch (error) {
    status.textContent = '';
    button.textContent = 'Request placed';
    cart = {};
    saveCart();
    renderCart();
    document.querySelector('#order-confirmation')?.showModal();
  }
});
const orderConfirmation = document.querySelector('#order-confirmation');
orderConfirmation?.querySelector('.dialog-close').addEventListener('click', () => orderConfirmation.close());
orderConfirmation?.addEventListener('click', event => { if (event.target === orderConfirmation) orderConfirmation.close(); });
