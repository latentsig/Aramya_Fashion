import { Link } from 'react-router-dom';
import { formatPrice, imageUrl } from '../features/products/catalog';
import { useCart } from '../features/cart/CartContext';

export function CartPage() {
  const { lines, subtotal, setQuantity, remove } = useCart();
  return (
    <main className="cart-page section-pad">
      <p className="eyebrow">A LITTLE SOMETHING</p><h1>Your bag</h1>
      {!lines.length ? <div className="cart-empty"><h2>Your bag is waiting.</h2><p>Explore the collections and save a piece for your next celebration.</p><Link className="button button-red" to="/collections">Explore collections ↗</Link></div> : <div className="cart-layout">
        <div className="cart-lines">{lines.map(({ product, quantity }) => <article className="cart-line" key={product.name}><img src={imageUrl(product.image)} alt={product.name} /><div className="cart-line-copy"><h2>{product.name}</h2><p>{formatPrice(product.priceRupees)}</p><label>Quantity <select aria-label={`Quantity for ${product.name}`} value={quantity} onChange={(event) => setQuantity(product.name, Number(event.target.value))}>{Array.from({ length: 10 }, (_, index) => index + 1).map((value) => <option key={value} value={value}>{value}</option>)}</select></label></div><button type="button" className="text-link" onClick={() => remove(product.name)}>Remove</button></article>)}</div>
        <aside className="cart-summary"><h2>Order summary</h2><p><span>Subtotal</span><strong>{formatPrice(subtotal)}</strong></p><small>Delivery, fitting and any customisation charges are confirmed by the studio.</small><Link className="button button-red" to="/checkout">Continue to checkout ↗</Link></aside>
      </div>}
    </main>
  );
}
