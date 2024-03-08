import { NgModule } from '@angular/core'
import { Routes, RouterModule, RouteReuseStrategy, ActivatedRouteSnapshot, DetachedRouteHandle } from '@angular/router'

import { AuthGuard } from './auth.guard'
import { DashboardComponent } from './dashboard/dashboard.component'
import { LoginScreenComponent } from './login-screen/login-screen.component'
import { SwaggerUiComponent } from './swagger-ui/swagger-ui.component'
import { MachinesPageComponent } from './machines-page/machines-page.component'
import { UsersPageComponent } from './users-page/users-page.component'
import { AppsPageComponent } from './apps-page/apps-page.component'
import { ProfilePageComponent } from './profile-page/profile-page.component'
import { PasswordChangePageComponent } from './password-change-page/password-change-page.component'
import { HostsPageComponent } from './hosts-page/hosts-page.component'
import { SubnetsPageComponent } from './subnets-page/subnets-page.component'
import { SharedNetworksPageComponent } from './shared-networks-page/shared-networks-page.component'
import { SettingsPageComponent } from './settings-page/settings-page.component'
import { EventsPageComponent } from './events-page/events-page.component'
import { ForbiddenPageComponent } from './forbidden-page/forbidden-page.component'
import { LogViewPageComponent } from './log-view-page/log-view-page.component'
import { LeaseSearchPageComponent } from './lease-search-page/lease-search-page.component'
import { KeaDaemonConfigurationPageComponent } from './kea-daemon-configuration-page/kea-daemon-configuration-page.component'
import { ConfigCheckerPreferencePageComponent } from './config-checker-preference-page/config-checker-preference-page.component'

const routes: Routes = [
    {
        path: '',
        pathMatch: 'full',
        redirectTo: 'dashboard',
    },
    {
        path: 'dashboard',
        component: DashboardComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'login',
        component: LoginScreenComponent,
    },
    {
        path: 'logout',
        component: LoginScreenComponent,
    },
    {
        path: 'machines',
        pathMatch: 'full',
        redirectTo: 'machines/all',
    },
    {
        path: 'machines/:id',
        component: MachinesPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'apps/:appType',
        pathMatch: 'full',
        redirectTo: 'apps/:appType/all',
    },
    {
        path: 'apps/:appType/:id',
        component: AppsPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'apps/kea/:appId/daemons/:daemonId/config',
        component: KeaDaemonConfigurationPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'dhcp/leases',
        component: LeaseSearchPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'dhcp/hosts',
        pathMatch: 'full',
        redirectTo: 'dhcp/hosts/all',
    },
    {
        path: 'dhcp/hosts/:id',
        component: HostsPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'dhcp/subnets',
        pathMatch: 'full',
        redirectTo: 'dhcp/subnets/all',
    },
    {
        path: 'dhcp/subnets/:id',
        component: SubnetsPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'dhcp/shared-networks',
        pathMatch: 'full',
        redirectTo: 'dhcp/shared-networks/all',
    },
    {
        path: 'dhcp/shared-networks/:id',
        component: SharedNetworksPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'swagger-ui',
        component: SwaggerUiComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'profile',
        component: ProfilePageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'profile/settings',
        component: ProfilePageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'profile/password',
        component: PasswordChangePageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'users',
        component: UsersPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'users/:id',
        component: UsersPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'users/new',
        component: UsersPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'settings',
        component: SettingsPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'events',
        component: EventsPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'forbidden',
        component: ForbiddenPageComponent,
    },
    {
        path: 'logs/:id',
        component: LogViewPageComponent,
        canActivate: [AuthGuard],
    },
    {
        path: 'review-checkers',
        component: ConfigCheckerPreferencePageComponent,
        canActivate: [AuthGuard],
    },

    // otherwise redirect to home
    { path: '**', redirectTo: '/' },
]

@NgModule({
    imports: [RouterModule.forRoot(routes, {})],
    exports: [RouterModule],
})
export class AppRoutingModule {}

export class CustomRouteReuseStrategy implements RouteReuseStrategy {
    /**
     * The point of this CustomRouteReuseStrategy is to skip route reuse in specific cases.
     * Hence, this method is not implemented. Nothing will be retrieved.
     * @param route previously stored route
     */
    retrieve(route: ActivatedRouteSnapshot): DetachedRouteHandle | null {
        return null
    }

    /**
     * The point of this CustomRouteReuseStrategy is to skip route reuse in specific cases.
     * Hence, this method always returns false.
     * @param route route to be attached
     */
    shouldAttach(route: ActivatedRouteSnapshot): boolean {
        return false
    }

    /**
     * The point of this CustomRouteReuseStrategy is to skip route reuse in specific cases.
     * Hence, this method always returns false.
     * @param route route to be detached
     */
    shouldDetach(route: ActivatedRouteSnapshot): boolean {
        return false
    }

    /**
     * Determines whether the route should be reused.
     * It returns false when navigation happens between HostsPageComponents
     * and when curr and future routes display list of hosts (tab index 0).
     * For other routes, true is returned whenever current route and future
     * route have exactly the same routeConfig. In this case, default Angular
     * route reuse strategy will work as usual.
     * @param future route to which we are trying to navigate
     * @param curr route from which we are leaving
     */
    shouldReuseRoute(future: ActivatedRouteSnapshot, curr: ActivatedRouteSnapshot): boolean {
        if (
            future.component === HostsPageComponent &&
            curr.component === HostsPageComponent &&
            curr.paramMap.get('id')?.includes('all') &&
            future.paramMap.get('id')?.includes('all')
        ) {
            // Do not reuse route when navigation happens between HostsPageComponents
            // and when curr and future routes display list of hosts (tab index 0).
            return false
        }
        return future.routeConfig === curr.routeConfig
    }

    /**
     * The point of this CustomRouteReuseStrategy is to skip route reuse in specific cases.
     * Hence, this method is not implemented. Nothing will be stored.
     * @param route route to be stored
     * @param handle detached route handle to be stored for reuse
     */
    store(route: ActivatedRouteSnapshot, handle: DetachedRouteHandle | null): void {
        // no-op
    }
}
