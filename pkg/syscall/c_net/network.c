#ifdef __cplusplus
extern "C"
{
#endif




#define _GNU_SOURCE

#include <stdio.h>

#undef _GNU_SOURCe

#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <string.h>
#include <stdio.h>
#include <ctype.h>
#include <time.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/param.h>
#include <sys/ioctl.h>
#include <arpa/inet.h>
#include <net/if.h>
#include <net/if_arp.h>
#include <net/ethernet.h>
#include <netinet/in.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <linux/sockios.h>

#include "network.h"
#include "nl.h"

#if HAVE_IFADDRS_H
#include <ifaddrs.h>
#else

#include <../include/ifaddrs.h>

#endif

#ifndef IFLA_LINKMODE
#  define IFLA_LINKMODE 17
#endif

#ifndef IFLA_LINKINFO
#  define IFLA_LINKINFO 18
#endif

#ifndef IFLA_NET_NS_PID
#  define IFLA_NET_NS_PID 19
#endif

#ifndef IFLA_INFO_KIND
# define IFLA_INFO_KIND 1
#endif

#ifndef IFLA_VLAN_ID
# define IFLA_VLAN_ID 1
#endif

#ifndef IFLA_INFO_DATA
#  define IFLA_INFO_DATA 2
#endif

#ifndef VETH_INFO_PEER
# define VETH_INFO_PEER 1
#endif

#ifndef IFLA_MACVLAN_MODE
# define IFLA_MACVLAN_MODE 1
#endif

int netdev_set_flag(const char *name, int flag) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct ifinfomsg *ifi;
    int index, len, err;

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -EINVAL;
    len = strlen(name);
    if (len == 1 || len >= IFNAMSIZ)
        goto out;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    err = -EINVAL;
    index = if_nametoindex(name);
    if (!index)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags = NLM_F_REQUEST | NLM_F_ACK;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWLINK;

    ifi = nlmsg_reserve(nlmsg, sizeof(struct ifinfomsg));
    ifi->ifi_family = AF_UNSPEC;
    ifi->ifi_index = index;
    ifi->ifi_change |= IFF_UP;
    ifi->ifi_flags |= flag;

    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(nlmsg);
    nlmsg_free(answer);
    return err;
}

int lxc_netdev_up(const char *name) {
    return netdev_set_flag(name, IFF_UP);
}

int lxc_netdev_down(const char *name) {
    return netdev_set_flag(name, 0);
}

int lxc_veth_create(const char *name1, const char *name2) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct ifinfomsg *ifi;
    struct rtattr *nest1, *nest2, *nest3;
    int len, err;

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -EINVAL;
    len = strlen(name1);
    if (len == 1 || len >= IFNAMSIZ)
        goto out;

    len = strlen(name2);
    if (len == 1 || len >= IFNAMSIZ)
        goto out;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags =
            NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL | NLM_F_ACK;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWLINK;

    ifi = nlmsg_reserve(nlmsg, sizeof(struct ifinfomsg));
    ifi->ifi_family = AF_UNSPEC;

    err = -EINVAL;
    nest1 = nla_begin_nested(nlmsg, IFLA_LINKINFO);
    if (!nest1)
        goto out;

    if (nla_put_string(nlmsg, IFLA_INFO_KIND, "veth"))
        goto out;

    nest2 = nla_begin_nested(nlmsg, IFLA_INFO_DATA);
    if (!nest2)
        goto out;

    nest3 = nla_begin_nested(nlmsg, VETH_INFO_PEER);
    if (!nest3)
        goto out;

    ifi = nlmsg_reserve(nlmsg, sizeof(struct ifinfomsg));
    if (!ifi)
        goto out;

    if (nla_put_string(nlmsg, IFLA_IFNAME, name2))
        goto out;

    nla_end_nested(nlmsg, nest3);

    nla_end_nested(nlmsg, nest2);

    nla_end_nested(nlmsg, nest1);

    if (nla_put_string(nlmsg, IFLA_IFNAME, name1))
        goto out;

    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(answer);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_netdev_move_by_index(int ifindex, pid_t pid, const char *ifname) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL;
    struct ifinfomsg *ifi;
    int err;

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags = NLM_F_REQUEST | NLM_F_ACK;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWLINK;

    ifi = nlmsg_reserve(nlmsg, sizeof(struct ifinfomsg));
    ifi->ifi_family = AF_UNSPEC;
    ifi->ifi_index = ifindex;

    if (nla_put_u32(nlmsg, IFLA_NET_NS_PID, pid))
        goto out;

    if (ifname != NULL) {
        if (nla_put_string(nlmsg, IFLA_IFNAME, ifname))
            goto out;
    }

    err = netlink_transaction(&nlh, nlmsg, nlmsg);
    out:
    netlink_close(&nlh);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_netdev_move_by_name(const char *ifname, pid_t pid, const char *newname) {
    int index;
    char *physname;

    if (!ifname)
        return -EINVAL;

    index = if_nametoindex(ifname);
    if (!index)
        return -EINVAL;

    return lxc_netdev_move_by_index(index, pid, newname);
}

int setup_private_host_hw_addr(char *veth1) {
    struct ifreq ifr;
    int err;
    int sockfd;

    sockfd = socket(AF_INET, SOCK_DGRAM, 0);
    if (sockfd < 0)
        return errno;

    snprintf((char *) ifr.ifr_name, IFNAMSIZ, "%s", veth1);
    err = ioctl(sockfd, SIOCGIFHWADDR, &ifr);
    if (err < 0) {
        close(sockfd);
        return errno;
    }

    ifr.ifr_hwaddr.sa_data[0] = 0xfe;
    err = ioctl(sockfd, SIOCSIFHWADDR, &ifr);
    close(sockfd);
    if (err < 0)
        return errno;

    return 0;
}

int lxc_bridge_attach(const char *bridge, const char *ifname) {
    int fd, index, err;
    struct ifreq ifr;

    if (strlen(ifname) >= IFNAMSIZ)
        return -EINVAL;

    index = if_nametoindex(ifname);
    if (!index)
        return -EINVAL;

    fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0)
        return errno;

    strncpy(ifr.ifr_name, bridge, IFNAMSIZ - 1);
    ifr.ifr_name[IFNAMSIZ - 1] = '\0';
    ifr.ifr_ifindex = index;
    err = ioctl(fd, SIOCBRADDIF, &ifr);
    close(fd);
    if (err < 0)
        err = errno;

    return err;
}

int lxc_convert_mac(char *macaddr, struct sockaddr *sockaddr) {
    unsigned char *data;
    char c;
    int i = 0;
    unsigned val;

    sockaddr->sa_family = ARPHRD_ETHER;
    data = (unsigned char *) sockaddr->sa_data;

    while ((*macaddr != '\0') && (i < ETH_ALEN)) {
        val = 0;
        c = *macaddr++;
        if (isdigit(c))
            val = c - '0';
        else if (c >= 'a' && c <= 'f')
            val = c - 'a' + 10;
        else if (c >= 'A' && c <= 'F')
            val = c - 'A' + 10;
        else {
            return -EINVAL;
        }
        val <<= 4;
        c = *macaddr;
        if (isdigit(c))
            val |= c - '0';
        else if (c >= 'a' && c <= 'f')
            val |= c - 'a' + 10;
        else if (c >= 'A' && c <= 'F')
            val |= c - 'A' + 10;
        else if (c == ':' || c == 0)
            val >>= 4;
        else {
            return -EINVAL;
        }
        if (c != 0)
            macaddr++;
        *data++ = (unsigned char) (val & 0377);
        i++;

        if (*macaddr == ':')
            macaddr++;
    }

    return 0;
}

static int ip_addr_add(int family, int ifindex,
                       void *addr, void *bcast, void *acast, int prefix) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct ifaddrmsg *ifa;
    int addrlen;
    int err;

    addrlen = family == AF_INET ? sizeof(struct in_addr) :
              sizeof(struct in6_addr);

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags =
            NLM_F_ACK | NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWADDR;

    ifa = nlmsg_reserve(nlmsg, sizeof(struct ifaddrmsg));
    ifa->ifa_prefixlen = prefix;
    ifa->ifa_index = ifindex;
    ifa->ifa_family = family;
    ifa->ifa_scope = 0;

    err = -EINVAL;
    if (nla_put_buffer(nlmsg, IFA_LOCAL, addr, addrlen))
        goto out;

    if (nla_put_buffer(nlmsg, IFA_ADDRESS, addr, addrlen))
        goto out;

    if (nla_put_buffer(nlmsg, IFA_BROADCAST, bcast, addrlen))
        goto out;

    /* TODO : multicast, anycast with ipv6 */
    err = -EPROTONOSUPPORT;
    if (family == AF_INET6 &&
        (memcmp(bcast, &in6addr_any, sizeof(in6addr_any)) ||
         memcmp(acast, &in6addr_any, sizeof(in6addr_any))))
        goto out;

    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(answer);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_ipv4_addr_add(int ifindex, struct in_addr *addr,
                      struct in_addr *bcast, int prefix) {
    return ip_addr_add(AF_INET, ifindex, addr, bcast, NULL, prefix);
}

static int ip_gateway_add(int family, int ifindex, void *gw) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct rtmsg *rt;
    int addrlen;
    int err;

    addrlen = family == AF_INET ? sizeof(struct in_addr) :
              sizeof(struct in6_addr);

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags =
            NLM_F_ACK | NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWROUTE;

    rt = nlmsg_reserve(nlmsg, sizeof(struct rtmsg));
    rt->rtm_family = family;
    rt->rtm_table = RT_TABLE_MAIN;
    rt->rtm_scope = RT_SCOPE_UNIVERSE;
    rt->rtm_protocol = RTPROT_BOOT;
    rt->rtm_type = RTN_UNICAST;
    /* "default" destination */
    rt->rtm_dst_len = 0;

    err = -EINVAL;
    if (nla_put_buffer(nlmsg, RTA_GATEWAY, gw, addrlen))
        goto out;

    /* Adding the interface index enables the use of link-local
     * addresses for the gateway */
    if (nla_put_u32(nlmsg, RTA_OIF, ifindex))
        goto out;

    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(answer);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_ipv4_gateway_add(int ifindex, struct in_addr *gw) {
    return ip_gateway_add(AF_INET, ifindex, gw);
}

static int ip_route_dest_add(int family, int ifindex, void *dest) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct rtmsg *rt;
    int addrlen;
    int err;

    addrlen = family == AF_INET ? sizeof(struct in_addr) :
              sizeof(struct in6_addr);

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags =
            NLM_F_ACK | NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL;
    nlmsg->nlmsghdr->nlmsg_type = RTM_NEWROUTE;

    rt = nlmsg_reserve(nlmsg, sizeof(struct rtmsg));
    rt->rtm_family = family;
    rt->rtm_table = RT_TABLE_MAIN;
    rt->rtm_scope = RT_SCOPE_LINK;
    rt->rtm_protocol = RTPROT_BOOT;
    rt->rtm_type = RTN_UNICAST;
    rt->rtm_dst_len = addrlen * 8;

    err = -EINVAL;
    if (nla_put_buffer(nlmsg, RTA_DST, dest, addrlen))
        goto out;
    if (nla_put_u32(nlmsg, RTA_OIF, ifindex))
        goto out;
    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(answer);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_ipv4_dest_add(int ifindex, struct in_addr *dest) {
    return ip_route_dest_add(AF_INET, ifindex, dest);
}

int setup_hw_addr(char *hwaddr, const char *ifname) {
    struct sockaddr sockaddr;
    struct ifreq ifr;
    int ret, fd;

    ret = lxc_convert_mac(hwaddr, &sockaddr);
    if (ret) {
        printf("mac address '%s' conversion failed : %d`\n",
               hwaddr, -ret);
        return -1;
    }

    memcpy(ifr.ifr_name, ifname, IFNAMSIZ);
    ifr.ifr_name[IFNAMSIZ - 1] = '\0';
    memcpy((char *) &ifr.ifr_hwaddr, (char *) &sockaddr, sizeof(sockaddr));

    fd = socket(AF_INET, SOCK_DGRAM, 0);
    if (fd < 0) {
        printf("socket failure : %d\n", errno);
        return -1;
    }

    ret = ioctl(fd, SIOCSIFHWADDR, &ifr);
    close(fd);
    if (ret)
        printf("ioctl failure : %d\n", errno);

    //printf("mac address '%s' on '%s' has been setup\n", hwaddr, ifr.ifr_name);
    return ret;
}

static const char padchar[] =
        "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";

char *lxc_mkifname(char *template)
{
    char *name = NULL;
    int i = 0;
    FILE *urandom;
    unsigned int seed;
    struct ifaddrs *ifaddr, *ifa;
    int ifexists = 0;

    /* Get all the network interfaces */
    getifaddrs(&ifaddr);

    /* Initialize the random number generator */
    urandom = fopen("/dev/urandom", "r");
    if (urandom != NULL) {
        if (fread (&seed,  sizeof(seed), 1, urandom) <= 0)
            seed = time(0);
        fclose(urandom);
    }else
    seed = time(0);

    #ifndef HAVE_RAND_R
    srand(seed);
    #endif

    /* Generate random names until we find one that doesn't exist */
    while (1) {
        ifexists = 0;
        name = strdup(
        template);

        if (name == NULL)
            return NULL;

    for ( i = 0; i< strlen(name); i++) {
        if (name[i] == 'X') {
            #ifdef HAVE_RAND_R
            name[i] = padchar[rand_r(&seed) % (strlen(padchar) - 1)];
            #else
            name[i] = padchar[rand()% (strlen(padchar)- 1)];
            #endif
        }
    }

    for (ifa = ifaddr;ifa != NULL;ifa = ifa->ifa_next ) {
        if (strcmp(ifa->ifa_name, name) == 0) {
            ifexists = 1;
            break;
        }
    }

    if (ifexists == 0)
        break;

    free(name);
   }

    freeifaddrs(ifaddr);
    return
    name;
}

int lxc_netdev_delete_by_index(int ifindex) {
    struct nl_handler nlh;
    struct nlmsg *nlmsg = NULL, *answer = NULL;
    struct ifinfomsg *ifi;
    int err;

    err = netlink_open(&nlh, NETLINK_ROUTE);
    if (err)
        return err;

    err = -ENOMEM;
    nlmsg = nlmsg_alloc(NLMSG_GOOD_SIZE);
    if (!nlmsg)
        goto out;

    answer = nlmsg_alloc_reserve(NLMSG_GOOD_SIZE);
    if (!answer)
        goto out;

    nlmsg->nlmsghdr->nlmsg_flags = NLM_F_ACK | NLM_F_REQUEST;
    nlmsg->nlmsghdr->nlmsg_type = RTM_DELLINK;

    ifi = nlmsg_reserve(nlmsg, sizeof(struct ifinfomsg));
    ifi->ifi_family = AF_UNSPEC;
    ifi->ifi_index = ifindex;

    err = netlink_transaction(&nlh, nlmsg, answer);
    out:
    netlink_close(&nlh);
    nlmsg_free(answer);
    nlmsg_free(nlmsg);
    return err;
}

int lxc_netdev_delete_by_name(const char *name) {
    int index;

    index = if_nametoindex(name);
    if (!index)
        return -EINVAL;

    return lxc_netdev_delete_by_index(index);
}

void new_hwaddr(char *hwaddr) {
    FILE *f;
    f = fopen("/dev/urandom", "r");
    if (f) {
        unsigned int seed;
        int ret = fread(&seed, sizeof(seed), 1, f);
        if (ret != 1)
            seed = time(NULL);
        fclose(f);
        srand(seed);
    } else
        srand(time(NULL));
    snprintf(hwaddr, 18, "00:16:3e:%02x:%02x:%02x",
             rand() % 255, rand() % 255, rand() % 255);
}

char *get_hardware_addr(char * name){
	struct ifreq ifr;

	int fd;
	if((fd = socket(AF_INET,SOCK_STREAM,0)) < 0){
		perror("socket");
		return NULL;
	}
	strcpy(ifr.ifr_name,name);

	if(ioctl(fd,SIOCGIFHWADDR,&ifr) < 0){
		perror("ioctl");
		return NULL;
	}

	char *macaddr = (char *)malloc(6);
	strncpy(macaddr,ifr.ifr_hwaddr.sa_data,6);
	macaddr[6] = '\0';
	return macaddr;
}



#ifdef __cplusplus
}
#endif