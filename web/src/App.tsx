import { Outlet, Link, NavLink } from 'react-router-dom';

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16 items-center">
            <Link to="/" className="text-xl font-bold text-gray-900">
              Taiwan AI Ecosystem Registry
            </Link>
            <nav className="flex space-x-8">
              <NavLink to="/" className="text-gray-600 hover:text-gray-900">
                Dashboard
              </NavLink>
              <NavLink to="/servers" className="text-gray-600 hover:text-gray-900">
                Servers
              </NavLink>
              <NavLink to="/search" className="text-gray-600 hover:text-gray-900">
                Search
              </NavLink>
            </nav>
          </div>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet />
      </main>
    </div>
  );
}
