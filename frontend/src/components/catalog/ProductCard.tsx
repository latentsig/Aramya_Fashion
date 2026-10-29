import { formatPrice, imageUrl, type Product } from '../../features/products/catalog';
import { useCart } from '../../features/cart/CartContext';

export function ProductCard({ product }: { product: Product }) {
  const { add } = useCart();
  return (
    <article className="product-card">
      <div className="product-image">
        <img src={imageUrl(product.image)} alt={product.name} loading="lazy" />
        <span className="product-tag">{product.label}</span>
        <button className="quick-add" type="button" onClick={() => add(product)} aria-label={`Add ${product.name} to bag`}>Add to bag <span aria-hidden="true">+</span></button>
      </div>
      <div className="product-info"><div><span className="product-name">{product.name}</span><span className="product-category">Made to order · Custom fit available</span></div><span className="product-price">{formatPrice(product.priceRupees)}</span></div>
    </article>
  );
}
