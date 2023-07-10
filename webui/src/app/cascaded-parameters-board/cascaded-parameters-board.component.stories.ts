import { moduleMetadata, Meta, Story } from '@storybook/angular'
import { CascadedParametersBoardComponent } from './cascaded-parameters-board.component'
import { NoopAnimationsModule } from '@angular/platform-browser/animations'
import { TableModule } from 'primeng/table'
import { ButtonModule } from 'primeng/button'
import { LocalSubnet } from '../backend/model/localSubnet'

export default {
    title: 'App/CascadedParametersBoard',
    component: CascadedParametersBoardComponent,
    decorators: [
        moduleMetadata({
            imports: [ButtonModule, NoopAnimationsModule, TableModule],
            declarations: [CascadedParametersBoardComponent],
            providers: [],
        }),
    ],
} as Meta

const Template: Story<CascadedParametersBoardComponent<LocalSubnet>> = (
    args: CascadedParametersBoardComponent<LocalSubnet>
) => ({
    props: args,
})

export const SameParameters = Template.bind({})
SameParameters.args = {
    levels: ['Subnet', 'Shared Network', 'Global'],
    data: [
        {
            name: 'Server1',
            parameters: [
                {
                    cacheThreshold: 0.25,
                    cacheMaxAge: 1000,
                    clientClass: 'baz',
                    requireClientClasses: ['foo', 'bar'],
                    ddnsGeneratedPrefix: 'myhost',
                    ddnsOverrideClientUpdate: true,
                },
                {
                    cacheThreshold: 0.25,
                    cacheMaxAge: 1000,
                    clientClass: 'fbi',
                    requireClientClasses: ['abc'],
                    ddnsGeneratedPrefix: 'his',
                    ddnsOverrideClientUpdate: false,
                },
                {
                    cacheMaxAge: 1000,
                    requireClientClasses: ['abc'],
                    ddnsGeneratedPrefix: 'example',
                    ddnsOverrideClientUpdate: true,
                },
            ],
        },
        {
            name: 'Server2',
            parameters: [
                {
                    cacheThreshold: 0.22,
                    cacheMaxAge: 900,
                    clientClass: 'abc',
                    requireClientClasses: ['bar'],
                    ddnsGeneratedPrefix: 'hishost',
                    ddnsOverrideClientUpdate: true,
                },
                {
                    cacheThreshold: 0.21,
                    cacheMaxAge: 800,
                    clientClass: 'ibi',
                    requireClientClasses: ['abc', 'dec'],
                    ddnsGeneratedPrefix: 'her',
                    ddnsOverrideClientUpdate: true,
                },
                {
                    cacheMaxAge: 1000,
                    requireClientClasses: ['aaa'],
                    ddnsGeneratedPrefix: 'ours',
                    ddnsOverrideClientUpdate: false,
                },
            ],
        },
    ],
}

export const DistinctParameters = Template.bind({})
DistinctParameters.args = {
    levels: ['Subnet', 'Global'],
    data: [
        {
            name: 'Server1',
            parameters: [
                {
                    cacheThreshold: 0.25,
                },
                {
                    cacheMaxAge: 1000,
                },
            ],
        },
        {
            name: 'Server2',
            parameters: [
                {
                    clientClass: 'abc',
                },
                {
                    requireClientClasses: ['abc', 'dec'],
                },
            ],
        },
    ],
}