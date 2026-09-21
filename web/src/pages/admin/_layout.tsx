import { Outlet } from "react-router-dom";

import AdminPanelBar from "../../components/admin/AdminPanelBar";
import { AdminNavigationProvider } from "@/contexts/AdminNavigationContext";
import { AccountProvider } from "@/contexts/AccountContext";

const AdminLayout = () => {
  return (
    <AccountProvider>
      <AdminNavigationProvider>
        <AdminPanelBar content={<Outlet />} />
      </AdminNavigationProvider>
    </AccountProvider>
  );
};

export default AdminLayout;
