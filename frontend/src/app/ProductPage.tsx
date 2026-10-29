import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useCart } from '../features/cart/CartContext';
import { categories, formatPrice, imageUrl } from '../features/products/catalog';
import { fetchCatalogProduct } from '../features/products/api';

export function ProductPage() {
  const { slug = '' } = useParams();
  const { add, loading: cartLoading, error: cartError } = useCart();
  const [selectedVariantId, setSelectedVariantId] = useState<string | null>(null);
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);
  const [feedback, setFeedback] = useState('');
  const productQuery = useQuery({
    queryKey: ['catalog-product', slug],
    queryFn: ({ signal }) => fetchCatalogProduct(slug, signal),
    staleTime: 60_000,
    retry: 1,
  });

  if (productQuery.isPending) return <main className="product-detail-state section-pad" role="status">Loading this style…</main>;
  if (productQuery.isError || !productQuery.data) {
    return <main className="product-detail-state section-pad"><p className="eyebrow">ARAMYA COLLECTIONS</p><h1>We couldn’t load this style.</h1><p role="alert">{productQuery.error instanceof Error ? productQuery.error.message : 'The product service is unavailable.'}</p><button className="button button-red" type="button" onClick={() => void productQuery.refetch()}>Try again</button></main>;
  }

  const product = productQuery.data;
  const category = categories.find((item) => item.id === product.category);
  const variant = product.variants.find((item) => item.id === selectedVariantId)
    ?? product.variants.find((item) => item.isDefault)
    ?? product.variants[0];
  const images = product.images.length ? product.images : [{ imageKey: product.image, altText: product.name }];
  const activeImage = images[selectedImageIndex] ?? images[0];
  const productPrice = product.priceRupees + (variant?.priceAdjustmentPaise ?? 0) / 100;

  async function addSelectedStyle() {
    if (!variant || !variant.available) return;
    setFeedback('');
    const added = await add({ ...product, variantId: variant.id, priceRupees: productPrice });
    if (added) setFeedback('Added to your bag.');
  }

  return (
    <main className="product-detail section-pad">
      <nav className="product-breadcrumb" aria-label="Breadcrumb">
        <Link to="/collections">Collections</Link><span aria-hidden="true">/</span>
        {category ? <Link to={`/collections/${category.id}`}>{category.title}</Link> : <Link to="/collections">The collection</Link>}
        <span aria-hidden="true">/</span><span aria-current="page">{product.name}</span>
      </nav>
      <div className="product-detail-layout">
        <section className="product-gallery" aria-label={`${product.name} images`}>
          <div className="product-detail-image"><img src={imageUrl(activeImage.imageKey.split(/[\\/]/).at(-1) ?? activeImage.imageKey)} alt={activeImage.altText || product.name} fetchPriority="high" /></div>
          {images.length > 1 && <div className="product-gallery-thumbnails">{images.map((image, index) => <button key={`${image.imageKey}-${index}`} type="button" className={index === selectedImageIndex ? 'selected' : ''} onClick={() => setSelectedImageIndex(index)} aria-label={`Show image ${index + 1}`} aria-pressed={index === selectedImageIndex}><img src={imageUrl(image.imageKey.split(/[\\/]/).at(-1) ?? image.imageKey)} alt="" /></button>)}</div>}
        </section>
        <section className="product-detail-copy">
          <p className="eyebrow">{product.label.toUpperCase()}</p>
          <h1>{product.name}</h1>
          <p className="product-detail-price">{formatPrice(productPrice)}</p>
          <p className="product-detail-description">{product.description || (product.inventoryPolicy === 'made_to_order' ? 'Made to measure through the Aramya studio. Contact us to discuss fit and timing for your occasion.' : 'A considered style from the Aramya collection.')}</p>
          {product.variants.length > 0 ? <fieldset className="variant-picker">
            <legend>{variant?.optionName ?? 'Style option'}</legend>
            <div className="variant-options">{product.variants.map((option) => <label key={option.id} className={option.id === variant?.id ? 'variant-option selected' : 'variant-option'}>
              <input type="radio" name="product-variant" value={option.id} checked={option.id === variant?.id} disabled={!option.available} onChange={() => { setSelectedVariantId(option.id); setFeedback(''); }} />
              <span>{option.optionValue}</span>{!option.available && <small>Unavailable</small>}
            </label>)}</div>
          </fieldset> : <p className="product-availability" role="status">This style is not available to order right now.</p>}
          <p className="product-availability">{product.inventoryPolicy === 'made_to_order' ? 'Made to order · Custom fit available' : product.inventoryPolicy === 'tracked' ? (variant?.available ? 'Available to order' : 'Currently unavailable') : 'Availability confirmed at checkout'}</p>
          {cartError && <p className="product-action-error" role="alert">{cartError}</p>}
          {feedback && <p className="product-action-success" role="status">{feedback} <Link to="/cart">View bag ↗</Link></p>}
          <button className="button button-red product-add-button" type="button" onClick={() => void addSelectedStyle()} disabled={!variant || !variant.available || cartLoading}>
            {cartLoading ? 'Loading bag…' : variant?.available ? 'Add to bag' : 'Unavailable'} <span aria-hidden="true">↗</span>
          </button>
          <p className="product-detail-note">Your final delivery, fitting and customisation details are confirmed with the studio during checkout.</p>
        </section>
      </div>
    </main>
  );
}
