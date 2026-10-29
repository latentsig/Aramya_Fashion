import { formatPrice, imageUrl, type Product } from '../../features/products/catalog';
import { useCart } from '../../features/cart/CartContext';
import { Link } from 'react-router-dom';

export function ProductCard({ product }: { product: Product }) {
  const { add, loading } = useCart();
  return (
    <article className="product-card">
      <div className="product-image">
        {product.slug ? <Link className="product-image-link" to={`/products/${encodeURIComponent(product.slug)}`} aria-label={`View ${product.name}`}><img src={imageUrl(product.image)} alt={product.name} loading="lazy" /><span className="product-tag">{product.label}</span></Link> : <><img src={imageUrl(product.image)} alt={product.name} loading="lazy" /><span className="product-tag">{product.label}</span></>}
        <button className="quick-add" type="button" onClick={() => void add(product)} disabled={!product.variantId || loading} aria-label={product.variantId ? `Add ${product.name} to bag` : `Live catalog required to add ${product.name} to bag`}>Add to bag <span aria-hidden="true">+</span></button>
      </div>
      <div className="product-info"><div>{product.slug ? <Link className="product-name product-name-link" to={`/products/${encodeURIComponent(product.slug)}`}>{product.name}</Link> : <span className="product-name">{product.name}</span>}<span className="product-category">Made to order · Custom fit available</span></div><span className="product-price">{formatPrice(product.priceRupees)}</span></div>
    </article>
  );
}
