import { Link } from 'react-router-dom';
import { useCart } from '../../features/cart/CartContext';

export function SiteHeader() {
  const { count, error } = useCart();
  return (
    <>
    <header className="site-header">
      <button className="menu-toggle" type="button" aria-label="Open navigation" aria-expanded="false">☰</button>
      <nav className="nav nav-left" aria-label="Main navigation"><a href="#collections">Collections</a><a href="#story">Our story</a><a href="#studio-location">Visit the studio</a></nav>
      <Link className="brand" to="/" aria-label="Aramya Fashion House home"><img src="/media/logo.png" alt="Aramya Fashion House by Shubha Bharat" /></Link>
      <nav className="nav nav-right" aria-label="Store navigation"><a href="/#appointment">Made to measure</a><a href="/#footer">Contact</a><Link className="bag-button" to="/cart" aria-label={`Shopping bag, ${count} items`}>Bag <span className="bag-count">{count}</span></Link></nav>
    </header>
    {error && <div className="cart-feedback" role="alert">{error}</div>}
    </>
  );
}
