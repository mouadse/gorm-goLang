import { Outlet } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import './App.css';

function App() {
  return (
    <div className="layout">
      <Sidebar />
      <main className="main-content">
        <div className="content-container">
          <Outlet />
        </div>
      </main>
    </div>
  );
}

export default App;
