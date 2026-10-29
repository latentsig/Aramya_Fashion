import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ProductCard } from '../components/catalog/ProductCard';
import { categories, products } from '../features/products/catalog';
import { fetchCatalogProducts } from '../features/products/api';

export function CollectionPage() {
  const { slug = '' } = useParams();
  const category = categories.find((item) => item.id === slug);
  const catalogQuery = useQuery({
    queryKey: ['catalog-products', slug],
    queryFn: ({ signal }) => fetchCatalogProducts(slug, signal),
    staleTime: 60_000,
    retry: 1,
  });
  const collectionProducts = catalogQuery.data ?? products.filter((product) => product.category === slug);

  if (!category) return <main className="section-pad"><h1>Collection not found</h1><Link to="/collections">Explore all collections ↗</Link></main>;

  return (
    <main>
      <section className="category-page section-pad">
        <Link className="breadcrumbs" to="/collections">COLLECTIONS <span>/</span> {category.title.toUpperCase()}</Link>
        <p className="eyebrow">MADE FOR YOUR MOMENTS</p>
        <h1>{category.title} for <em>your moments.</em></h1>
        <p className="category-lede">{category.subtitle}. Explore the edit, then visit our studio to make the fit your own.</p>
        <div className="category-page-note"><span>MADE TO ORDER</span><span>PERSONAL FIT AVAILABLE</span><a href="/#appointment">Plan a studio visit ↗</a></div>
      </section>
      <section className="category-products section-pad">
        <div className="section-heading"><div><p className="eyebrow">THE {category.title.toUpperCase()} EDIT</p><h2>Find the one <em>you love.</em></h2></div><Link className="text-link" to="/collections">All collections ↗</Link></div>
        {catalogQuery.isError && <div className="catalog-status"><p role="status">Live catalog is unavailable. Showing the original Aramya catalog.</p><button type="button" onClick={() => void catalogQuery.refetch()}>Retry live catalog</button></div>}
        <div className="product-grid">{collectionProducts.map((product) => <ProductCard key={product.name} product={product} />)}</div>
      </section>
    </main>
  );
}
