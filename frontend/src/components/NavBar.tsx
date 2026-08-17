import { NavLink } from "react-router";

export function NavBar() {
  return (
    <nav className="navbar">
      <span className="navbar__brand">SiteWatch</span>
      <NavLink to="/" end>
        Sites
      </NavLink>
      <NavLink to="/alerts">Alerts</NavLink>
    </nav>
  );
}
