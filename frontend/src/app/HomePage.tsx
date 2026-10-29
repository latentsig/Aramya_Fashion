import { Link } from 'react-router-dom';
import { categories, imageUrl, products } from '../features/products/catalog';

export function HomePage() {
  return (
    <main id="top">
      <section className="hero">
        <div className="hero-copy"><p className="eyebrow">ARAMYA · BY SHUBHA BHARAT</p><h1>Wear the<br /><em>moment.</em></h1><p className="hero-intro">Indian occasionwear, imagined with intention and made to feel entirely your own.</p><a className="button button-light" href="#collections">Explore the collection <span>↘</span></a><div className="hero-index"><span>01</span><span className="index-rule" /><span>CRAFTED FOR YOU</span></div></div>
        <div className="hero-image-wrap"><video className="hero-image hero-video" autoPlay muted loop playsInline preload="metadata" poster={imageUrl('lehanga2.jpg')} aria-label="A woman twirling in a colorful Indian saree"><source src="https://videos.pexels.com/video-files/7684805/7684805-hd_1920_1080_30fps.mp4" type="video/mp4" /></video><div className="image-note">A little wonder<br />in every detail.</div></div>
        <a className="hero-side-label" href="#story">OCCASIONWEAR · MADE WITH MEANING</a>
      </section>
      <div className="marquee"><div>MADE TO MEASURE <span>✳</span> ROOTED IN CULTURE <span>✳</span> READY FOR YOUR MOMENT <span>✳</span></div></div>
      <section className="intro section-pad" id="story"><p className="eyebrow">A HOUSE OF PERSONAL STYLE</p><h2>Tradition, with<br /><em>room to be you.</em></h2><p className="intro-copy">At Aramya, the best outfit is the one that feels like it was always yours. Discover thoughtful silhouettes, expressive colour and the quiet joy of a perfect fit—designed by Shubha Bharat for celebrations big and small.</p></section>
      <section className="collections section-pad" id="collections"><div className="section-heading"><div><p className="eyebrow">THE ARAMYA COLLECTIONS</p><h2>Find your <em>occasion.</em></h2></div><Link className="text-link" to="/collections">Explore all collections ↗</Link></div><p className="collections-intro">Explore a collection, then find the piece that feels like you. Every style can be tailored with our studio team.</p><div className="category-grid">{categories.map((category) => <Link className="category-card" to={`/collections/${category.id}`} key={category.id}><div className="category-image"><img src={imageUrl(category.image)} alt={`Explore ${category.title}`} /><span className="category-count">{products.filter((product) => product.category === category.id).length} styles</span></div><div className="category-card-copy"><div><h3>{category.title}</h3><p>{category.subtitle}</p></div><span className="category-arrow">↗</span></div></Link>)}</div></section>
      <section className="custom"><div className="custom-image"><img src={imageUrl('bridal lehanga.jpg')} alt="Full-length red bridal lehenga with detailed embroidery" /></div><div className="custom-copy"><p className="eyebrow">YOUR VISION, OUR CRAFT</p><h2>Some things are<br /><em>made just for you.</em></h2><p>Choose your colour, shape and finishing touches. We’ll work together to create a piece that fits you—and the moment—beautifully.</p><a className="button button-light" href="#custom-order">Start a custom order <span>↗</span></a></div></section>
      <section className="studio-location section-pad" id="studio-location"><div><p className="eyebrow">ROOTED IN THE COASTAL NORTH</p><h2>Visit us in <em>Ankola.</em></h2><p>Plan a relaxed studio visit to explore fabrics, talk through an occasion, and find a fit that feels just right.</p></div><div className="location-card"><span className="location-pin">A.</span><div><b>ARAMYA FASHION HOUSE</b><span>Ankola, Uttara Kannada,<br />Karnataka, India</span><span>Studio visits by appointment</span></div></div></section>
      <span id="appointment" />
    </main>
  );
}
