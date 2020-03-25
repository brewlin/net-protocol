#ifdef __cplusplus
extern "C"
{
#endif
#include <sys/types.h>
int netdev_set_flag(const char *name, int flag);
int lxc_netdev_up(const char *name);
int lxc_netdev_down(const char *name);
int lxc_veth_create(const char *name1, const char *name2);
int lxc_netdev_move_by_index(int ifindex, pid_t pid, const char* ifname);
int lxc_netdev_move_by_name(const char *ifname, pid_t pid, const char* newname);
int setup_private_host_hw_addr(char *veth1);
int lxc_bridge_attach(const char *bridge, const char *ifname);
int lxc_convert_mac(char *macaddr, struct sockaddr *sockaddr);
static int ip_addr_add(int family, int ifindex,
                       void *addr, void *bcast, void *acast, int prefix);
int lxc_ipv4_addr_add(int ifindex, struct in_addr *addr,
                      struct in_addr *bcast, int prefix);
static int ip_gateway_add(int family, int ifindex, void *gw);
int lxc_ipv4_gateway_add(int ifindex, struct in_addr *gw);
static int ip_route_dest_add(int family, int ifindex, void *dest);
int lxc_ipv4_dest_add(int ifindex, struct in_addr *dest);
int setup_hw_addr(char *hwaddr, const char *ifname);
char *lxc_mkifname(char *);
int lxc_netdev_delete_by_index(int ifindex);
int lxc_netdev_delete_by_name(const char *name);
void new_hwaddr(char *hwaddr);
//get mac addr
char *get_hardware_addr(char *name);


#ifdef __cplusplus
}
#endif