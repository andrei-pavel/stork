package keactrl

import keaconfig "isc.org/stork/appcfg/kea"

const (
	ListSubnets       CommandName = "list-subnets"
	Network4Add       CommandName = "network4-add"
	Network6Add       CommandName = "network6-add"
	Network4Del       CommandName = "network4-del"
	Network6Del       CommandName = "network6-del"
	Network4SubnetAdd CommandName = "network4-subnet-add"
	Network6SubnetAdd CommandName = "network6-subnet-add"
	Network4SubnetDel CommandName = "network4-subnet-del"
	Network6SubnetDel CommandName = "network6-subnet-del"
	Subnet4Add        CommandName = "subnet4-add"
	Subnet6Add        CommandName = "subnet6-add"
	Subnet4Del        CommandName = "subnet4-del"
	Subnet6Del        CommandName = "subnet6-del"
	Subnet4Get        CommandName = "subnet4-get"
	Subnet6Get        CommandName = "subnet6-get"
	Subnet4Update     CommandName = "subnet4-update"
	Subnet6Update     CommandName = "subnet6-update"
)

// Creates network4-add command.
func NewCommandNetwork4Add(sharedNetwork *keaconfig.SharedNetwork4, daemonNames ...string) *Command {
	return NewCommandBase(Network4Add, daemonNames...).WithArrayArgument("shared-networks", sharedNetwork)
}

// Creates network6-add command.
func NewCommandNetwork6Add(sharedNetwork *keaconfig.SharedNetwork6, daemonNames ...string) *Command {
	return NewCommandBase(Network6Add, daemonNames...).WithArrayArgument("shared-networks", sharedNetwork)
}

// Creates network4-del command.
func NewCommandNetwork4Del(sharedNetwork *keaconfig.SubnetCmdsDeletedSharedNetwork, daemonNames ...string) *Command {
	return NewCommandBase(Network4Del, daemonNames...).WithArguments(sharedNetwork)
}

// Creates network6-del command.
func NewCommandNetwork6Del(sharedNetwork *keaconfig.SubnetCmdsDeletedSharedNetwork, daemonNames ...string) *Command {
	return NewCommandBase(Network6Del, daemonNames...).WithArguments(sharedNetwork)
}

// Creates network4-subnet-add command.
func NewCommandNetwork4SubnetAdd(sharedNetworkName string, localSubnetID int64, daemonNames ...string) *Command {
	return NewCommandBase(Network4SubnetAdd, daemonNames...).
		WithArgument("id", localSubnetID).
		WithArgument("name", sharedNetworkName)
}

// Creates network6-subnet-add command.
func NewCommandNetwork6SubnetAdd(sharedNetworkName string, localSubnetID int64, daemonNames ...string) *Command {
	return NewCommandBase(Network6SubnetAdd, daemonNames...).
		WithArgument("id", localSubnetID).
		WithArgument("name", sharedNetworkName)
}

// Creates network4-subnet-del command.
func NewCommandNetwork4SubnetDel(sharedNetworkName string, localSubnetID int64, daemonNames ...string) *Command {
	return NewCommandBase(Network4SubnetDel, daemonNames...).
		WithArgument("id", localSubnetID).
		WithArgument("name", sharedNetworkName)
}

// Creates network6-subnet-del command.
func NewCommandNetwork6SubnetDel(sharedNetworkName string, localSubnetID int64, daemonNames ...string) *Command {
	return NewCommandBase(Network6SubnetDel, daemonNames...).
		WithArgument("id", localSubnetID).
		WithArgument("name", sharedNetworkName)
}

// Creates subnet4-add command.
func NewCommandSubnet4Add(subnet *keaconfig.Subnet4, daemonNames ...string) *Command {
	return NewCommandBase(Subnet4Add, daemonNames...).WithArrayArgument("subnet4", subnet)
}

// Creates subnet6-add command.
func NewCommandSubnet6Add(subnet *keaconfig.Subnet6, daemonNames ...string) *Command {
	return NewCommandBase(Subnet6Add, daemonNames...).WithArrayArgument("subnet6", subnet)
}

// Creates subnet4-del command.
func NewCommandSubnet4Del(subnet *keaconfig.SubnetCmdsDeletedSubnet, daemonNames ...string) *Command {
	return NewCommandBase(Subnet4Del, daemonNames...).
		WithArgument("id", subnet.ID)
}

// Creates subnet6-del command.
func NewCommandSubnet6Del(subnet *keaconfig.SubnetCmdsDeletedSubnet, daemonNames ...string) *Command {
	return NewCommandBase(Subnet6Del, daemonNames...).
		WithArgument("id", subnet.ID)
}

// Creates subnet4-update command.
func NewCommandSubnet4Update(subnet *keaconfig.Subnet4, daemonNames ...string) *Command {
	return NewCommandBase(Subnet4Update, daemonNames...).WithArrayArgument("subnet4", subnet)
}

// Creates subnet6-update command.
func NewCommandSubnet6Update(subnet *keaconfig.Subnet6, daemonNames ...string) *Command {
	return NewCommandBase(Subnet6Update, daemonNames...).WithArrayArgument("subnet6", subnet)
}
